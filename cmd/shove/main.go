package main

import (
	"context"
	"flag"
	"gitlab.com/pennersr/shove/internal/queue"
	"gitlab.com/pennersr/shove/internal/queue/memory"
	"gitlab.com/pennersr/shove/internal/queue/redis"
	"gitlab.com/pennersr/shove/internal/server"
	"gitlab.com/pennersr/shove/internal/services"
	"gitlab.com/pennersr/shove/internal/services/apns"
	"gitlab.com/pennersr/shove/internal/services/email"
	"gitlab.com/pennersr/shove/internal/services/fcm"
	"gitlab.com/pennersr/shove/internal/services/telegram"
	"gitlab.com/pennersr/shove/internal/services/webpush"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var apiAddr = flag.String("api-addr", ":8322", "API address to listen to")
var apnsCertificate = flag.String("apns-certificate-path", "", "APNS certificate path")
var apnsSandboxCertificate = flag.String("apns-sandbox-certificate-path", "", "APNS sandbox certificate path")
var fcmAPIKey = flag.String("fcm-api-key", "", "FCM API key")
var redisURL = flag.String("queue-redis", "", "Use Redis queue (Redis URL)")
var webPushVAPIDPublicKey = flag.String("webpush-vapid-public-key", "", "VAPID public key")
var webPushVAPIDPrivateKey = flag.String("webpush-vapid-private-key", "", "VAPID public key")
var telegramBotToken = flag.String("telegram-bot-token", "", "Telegram bot token")
var telegramWorkers = flag.Int("telegram-workers", 2, "The number of workers pushing Telegram messages")
var telegramRateAmount = flag.Int("telegram-rate-amount", 0, "Telegram max. rate (amount)")
var telegramRatePer = flag.Int("telegram-rate-per", 0, "Telegram max. rate (per seconds)")
var emailHost = flag.String("email-host", "", "Email host")
var emailPort = flag.Int("email-port", 25, "Email port")
var emailRateAmount = flag.Int("email-rate-amount", 0, "Email max. rate (amount)")
var emailRatePer = flag.Int("email-rate-per", 0, "Email max. rate (per seconds)")

func newServiceLog(prefix string) *log.Logger {
	return log.New(log.Writer(), prefix+": ", log.Flags())
}

func main() {
	log.SetFlags(log.Flags() | log.Lmsgprefix)
	flag.Parse()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	var qf queue.QueueFactory
	if *redisURL == "" {
		log.Println("Using non-persistent in-memory queue")
		qf = memory.MemoryQueueFactory{}
	} else {
		log.Println("Using Redis queue at", *redisURL)
		qf = redis.NewQueueFactory(*redisURL)
	}
	s := server.NewServer(*apiAddr, qf)

	if *apnsCertificate != "" {
		apns, err := apns.NewAPNS(*apnsCertificate, true, newServiceLog("apns"))
		if err != nil {
			log.Fatal("[ERROR] Setting up APNS service:", err)
		}
		if err := s.AddService(apns); err != nil {
			log.Fatal("[ERROR] Adding APNS service:", err)
		}
	}

	if *apnsSandboxCertificate != "" {
		apns, err := apns.NewAPNS(*apnsSandboxCertificate, false, newServiceLog("apns-sandbox"))
		if err != nil {
			log.Fatal("[ERROR] Setting up APNS sandbox service:", err)
		}
		if err := s.AddService(apns); err != nil {
			log.Fatal("[ERROR] Adding APNS sandbox service:", err)
		}
	}

	if *fcmAPIKey != "" {
		fcm, err := fcm.NewFCM(*fcmAPIKey, newServiceLog("fcm"))
		if err != nil {
			log.Fatal("[ERROR] Setting up FCM service:", err)
		}
		if err := s.AddService(fcm); err != nil {
			log.Fatal("[ERROR] Adding FCM service:", err)
		}
	}

	if *webPushVAPIDPrivateKey != "" {
		web, err := webpush.NewWebPush(*webPushVAPIDPublicKey, *webPushVAPIDPrivateKey, newServiceLog("webpush"))
		if err != nil {
			log.Fatal("[ERROR] Setting up WebPush service:", err)
		}
		if err := s.AddService(web); err != nil {
			log.Fatal("[ERROR] Adding WebPush service:", err)
		}
	}

	if *telegramBotToken != "" {
		tg, err := telegram.NewTelegramService(*telegramBotToken, newServiceLog("telegram"), *telegramWorkers, services.DigestConfig{
			RateMax: *telegramRateAmount,
			RatePer: time.Second * time.Duration(*telegramRatePer),
		})
		if err != nil {
			log.Fatal("[ERROR] Setting up Telegram service:", err)
		}
		if err := s.AddService(tg); err != nil {
			log.Fatal("[ERROR] Adding Telegram service:", err)
		}
	}

	if *emailHost != "" {
		config := email.EmailConfig{
			EmailHost: *emailHost,
			EmailPort: *emailPort,
			RateMax:   *emailRateAmount,
			RatePer:   time.Second * time.Duration(*emailRatePer),
			Log:       newServiceLog("email"),
		}
		email, err := email.NewEmailService(config)
		if err != nil {
			log.Fatal("[ERROR] Setting up email service:", err)
		}
		if err := s.AddService(email); err != nil {
			log.Fatal("[ERROR] Adding email service:", err)
		}
	}

	go func() {
		log.Println("Serving on", *apiAddr)
		err := s.Serve()
		if err != nil {
			log.Fatal("[ERROR] Serving:", err)
		}
	}()
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Shutdown(ctx)
	log.Println("Exiting")
}
