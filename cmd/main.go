package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"masscan/internal/banner"
	"masscan/internal/config"
	"masscan/internal/models"
	"masscan/internal/notifier"
	"masscan/internal/scanner"
	"masscan/internal/storage"
)

type BannerTask struct {
	IP   string
	Port int
}

func detectService(port int, banner string) string {
	switch port {
	case 22:
		return "SSH"
	case 80:
		return "HTTP"
	case 443:
		return "HTTPS"
	case 21:
		return "FTP"
	case 25:
		return "SMTP"
	case 3306:
		return "MySQL"
	}

	lowerBanner := strings.ToLower(banner)

	if strings.Contains(lowerBanner, "ssh") {
		return "SSH"
	}
	if strings.Contains(lowerBanner, "http") {
		return "HTTP"
	}
	if strings.Contains(lowerBanner, "smtp") {
		return "SMTP"
	}
	if strings.Contains(lowerBanner, "ftp") {
		return "FTP"
	}

	return "Unknown"
}

func exists(old []models.ScanResult, r models.ScanResult) bool {
	for _, o := range old {
		if o.IP == r.IP && o.Port == r.Port {
			return true
		}
	}
	return false
}

func run(ctx context.Context) error {
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	store, err := storage.New(cfg.Storage.DBPath)
	if err != nil {
		return fmt.Errorf("failed to init storage: %w", err)
	}
	defer store.Close()

	var notifiers []notifier.Notifier

	if cfg.Telegram.Enabled {
		notifiers = append(notifiers, &notifier.TelegramNotifier{
			Token:  cfg.TelegramToken,
			ChatID: cfg.Telegram.ChatID,
		})
	}

	if cfg.Email.Enabled {
		notifiers = append(notifiers, &notifier.EmailNotifier{
			APIToken: cfg.MailtrapToken,
			InboxID:  cfg.MailtrapInboxID,
			From:     cfg.EmailFrom,
			To:       cfg.Email.To,
		})
	}

	fmt.Println("Starting scan...")

	var files []string
	defer func() {
		for _, file := range files {
			_ = os.Remove(file)
		}
	}()

	for i, target := range cfg.Scanner.Targets {
		select {
		case <-ctx.Done():
			return fmt.Errorf("scan canceled before target %s: %w", target, ctx.Err())
		default:
		}

		file := fmt.Sprintf("scan_%d.json", i)

		err := scanner.RunMasscan(
			ctx,
			cfg.Masscan.Path,
			target,
			cfg.Scanner.Ports,
			cfg.Scanner.Rate,
			file,
		)
		if err != nil {
			if ctx.Err() != nil {
				return fmt.Errorf("masscan canceled for target %s: %w", target, ctx.Err())
			}
			return fmt.Errorf("masscan failed for target %s: %w", target, err)
		}

		files = append(files, file)
	}

	if err := store.ImportLegacyJSONIfEmpty(ctx, cfg.Storage.File); err != nil {
		return fmt.Errorf("failed to import legacy results: %w", err)
	}

	oldResults, err := store.LoadCurrent(ctx)
	if err != nil {
		return fmt.Errorf("failed to load previous results: %w", err)
	}

	tasks := make(chan BannerTask, cfg.Workers.QueueSize)
	resultsChan := make(chan models.ScanResult, cfg.Workers.QueueSize)

	var workerWG sync.WaitGroup

	for i := 0; i < cfg.Workers.Count; i++ {
		workerWG.Add(1)

		go func() {
			defer workerWG.Done()

			for {
				select {
				case <-ctx.Done():
					return
				case task, ok := <-tasks:
					if !ok {
						return
					}

					bannerText := banner.GrabBanner(task.IP, task.Port)
					service := detectService(task.Port, bannerText)

					result := models.ScanResult{
						IP:      task.IP,
						Port:    task.Port,
						Service: service,
						Banner:  bannerText,
					}

					select {
					case <-ctx.Done():
						return
					case resultsChan <- result:
					}
				}
			}
		}()
	}

outer:
	for _, file := range files {
		results, err := scanner.ParseResults(file)
		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", file, err)
		}

		for _, r := range results {
			for _, p := range r.Ports {
				select {
				case <-ctx.Done():
					break outer
				case tasks <- BannerTask{IP: r.IP, Port: p.Port}:
				}
			}
		}
	}

	close(tasks)

	go func() {
		workerWG.Wait()
		close(resultsChan)
	}()

	var scanResults []models.ScanResult
	for res := range resultsChan {
		scanResults = append(scanResults, res)
	}

	if ctx.Err() != nil {
		return fmt.Errorf("scan interrupted: %w", ctx.Err())
	}

	fmt.Printf("Collected %d scan results\n", len(scanResults))

	for _, r := range scanResults {
		if !exists(oldResults, r) {
			msg := fmt.Sprintf(
				"New open service: %s:%d (%s)",
				r.IP,
				r.Port,
				r.Service,
			)

			fmt.Println(msg)

			for _, n := range notifiers {
				if err := n.Send(msg); err != nil {
					fmt.Printf("Notifier error: %T: %v\n", n, err)
				} else {
					fmt.Printf("Notifier success: %T\n", n)
				}
			}
		}
	}

	if err := store.ReplaceCurrent(ctx, scanResults); err != nil {
		return fmt.Errorf("failed to persist results to %s: %w", cfg.Storage.DBPath, err)
	}

	fmt.Println("Results saved to", cfg.Storage.DBPath)
	fmt.Println("Scan finished")

	return nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Println("Graceful shutdown complete")
			return
		}

		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
