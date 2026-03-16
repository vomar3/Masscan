package main

import (
	"fmt"
	"strings"
	"sync"

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

func main() {
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		panic(fmt.Errorf("failed to load config: %w", err))
	}

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

	for i, target := range cfg.Scanner.Targets {
		file := fmt.Sprintf("scan_%d.json", i)

		err := scanner.RunMasscan(
			cfg.Masscan.Path,
			target,
			cfg.Scanner.Ports,
			cfg.Scanner.Rate,
			file,
		)
		if err != nil {
			panic(fmt.Errorf("masscan failed for target %s: %w", target, err))
		}

		files = append(files, file)
	}

	oldResults, err := storage.Load(cfg.Storage.File)
	if err != nil {
		fmt.Println("Warning: failed to load previous results:", err)
		oldResults = []models.ScanResult{}
	}

	tasks := make(chan BannerTask, cfg.Workers.QueueSize)
	resultsChan := make(chan models.ScanResult, cfg.Workers.QueueSize)

	var workerWG sync.WaitGroup

	for i := 0; i < cfg.Workers.Count; i++ {
		workerWG.Add(1)

		go func() {
			defer workerWG.Done()

			for task := range tasks {
				bannerText := banner.GrabBanner(task.IP, task.Port)
				service := detectService(task.Port, bannerText)

				resultsChan <- models.ScanResult{
					IP:      task.IP,
					Port:    task.Port,
					Service: service,
					Banner:  bannerText,
				}
			}
		}()
	}

	taskCount := 0

	for _, file := range files {
		results, err := scanner.ParseResults(file)
		if err != nil {
			panic(fmt.Errorf("failed to parse %s: %w", file, err))
		}

		for _, r := range results {
			for _, p := range r.Ports {
				tasks <- BannerTask{
					IP:   r.IP,
					Port: p.Port,
				}
				taskCount++
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

	if err := storage.Save(cfg.Storage.File, scanResults); err != nil {
		panic(fmt.Errorf("failed to save results to %s: %w", cfg.Storage.File, err))
	}

	fmt.Println("Results saved to", cfg.Storage.File)
	fmt.Println("Scan finished")
	_ = taskCount
}
