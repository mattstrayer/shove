package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/mattstrayer/shove/internal/queue"
	"github.com/mattstrayer/shove/internal/queue/memory"
	"github.com/mattstrayer/shove/internal/queue/redis"
	"github.com/mattstrayer/shove/internal/server"
	"github.com/mattstrayer/shove/internal/services"
	"github.com/mattstrayer/shove/internal/services/apns"
	"github.com/mattstrayer/shove/internal/services/email"
	"github.com/mattstrayer/shove/internal/services/fcm"
	"github.com/mattstrayer/shove/internal/services/telegram"
	"github.com/mattstrayer/shove/internal/services/webhook"
	"github.com/mattstrayer/shove/internal/services/webpush"
)

// from -> https://www.gmarik.info/blog/2019/12-factor-golang-flag-package/
func LookupEnvOrString(key string, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}

func LookupEnvOrInt(key string, defaultVal int) int {
	if val, ok := os.LookupEnv(key); ok {
		v, err := strconv.Atoi(val)
		if err != nil {

			log.Fatalf("LookupEnvOrInt[%s]: %v", key, err)
		}
		return v
	}
	return defaultVal
}

func LookupEnvOrBool(key string, defaultVal bool) bool {
	if val, ok := os.LookupEnv(key); ok {
		v, err := strconv.ParseBool(val)
		if err != nil {
			log.Fatalf("LookupEnvOrBool[%s]: %v", key, err)
		}
		return v
	}
	return defaultVal
}

var debug = flag.Bool("debug", LookupEnvOrBool("DEBUG", false), "Enable debug logging")
var apiAddr = flag.String("api-addr", LookupEnvOrString("API_ADDR", ":8322"), "API address to listen to")

var apnsWorkers = flag.Int("apns-workers", LookupEnvOrInt("APNS_WORKERS", 4), "The number of workers pushing APNS messages")

// this must be set as an environment variable
var googleApplicationCredentials = flag.String("google-application-credentials", LookupEnvOrString("GOOGLE_APPLICATION_CREDENTIALS", ""), "Google application credentials path")
var fcmWorkers = flag.Int("fcm-workers", LookupEnvOrInt("FCM_WORKERS", 4), "The number of workers pushing FCM messages")

var redisURL = flag.String("queue-redis", LookupEnvOrString("QUEUE_REDIS", ""), "Use Redis queue (Redis URL)")

var webhookWorkers = flag.Int("webhook-workers", LookupEnvOrInt("WEBHOOK_WORKERS", 0), "The number of workers pushing Webhook messages")

var webPushVAPIDPublicKey = flag.String("webpush-vapid-public-key", LookupEnvOrString("WEBPUSH_VAPID_PUBLIC_KEY", ""), "VAPID public key")
var webPushVAPIDPrivateKey = flag.String("webpush-vapid-private-key", LookupEnvOrString("WEBPUSH_VAPID_PRIVATE_KEY", ""), "VAPID public key")
var webPushWorkers = flag.Int("webpush-workers", LookupEnvOrInt("WEBPUSH_WORKERS", 8), "The number of workers pushing Web messages")

var telegramBotToken = flag.String("telegram-bot-token", LookupEnvOrString("TELEGRAM_BOT_TOKEN", ""), "Telegram bot token")
var telegramWorkers = flag.Int("telegram-workers", LookupEnvOrInt("TELEGRAM_WORKERS", 2), "The number of workers pushing Telegram messages")
var telegramRateAmount = flag.Int("telegram-rate-amount", LookupEnvOrInt("TELEGRAM_RATE_AMOUNT", 0), "Telegram max. rate (amount)")
var telegramRatePer = flag.Int("telegram-rate-per", LookupEnvOrInt("TELEGRAM_RATE_PER", 0), "Telegram max. rate (per seconds)")

var emailHost = flag.String("email-host", LookupEnvOrString("EMAIL_HOST", ""), "Email host")
var emailPort = flag.Int("email-port", LookupEnvOrInt("EMAIL_PORT", 25), "Email port")
var emailPlainAuth = flag.Bool("email-plain-auth", LookupEnvOrBool("EMAIL_PLAIN_AUTH", false), "Email plain auth(username and password)")
var emailUsername = flag.String("email-username", LookupEnvOrString("EMAIL_USERNAME", ""), "Email username")
var emailPassword = flag.String("email-password", LookupEnvOrString("EMAIL_PASSWORD", ""), "Email password")
var emailTLS = flag.Bool("email-tls", LookupEnvOrBool("EMAIL_TLS", false), "Use TLS")
var emailTLSInsecure = flag.Bool("email-tls-insecure", LookupEnvOrBool("EMAIL_TLS_INSECURE", false), "Skip TLS verification")
var emailRateAmount = flag.Int("email-rate-amount", LookupEnvOrInt("EMAIL_RATE_AMOUNT", 0), "Email max. rate (amount)")
var emailRatePer = flag.Int("email-rate-per", LookupEnvOrInt("EMAIL_RATE_PER", 0), "Email max. rate (per seconds)")

var (
	apnsAuthKeyPath        = flag.String("apns-auth-key-path", LookupEnvOrString("APNS_AUTH_KEY_PATH", ""), "APNS authentication key path (.p8 file)")
	apnsKeyID              = flag.String("apns-key-id", LookupEnvOrString("APNS_KEY_ID", ""), "APNS Key ID from Apple Developer account")
	apnsTeamID             = flag.String("apns-team-id", LookupEnvOrString("APNS_TEAM_ID", ""), "APNS Team ID from Apple Developer account")
	apnsSandboxAuthKeyPath = flag.String("apns-sandbox-auth-key-path", LookupEnvOrString("APNS_SANDBOX_AUTH_KEY_PATH", ""), "APNS sandbox authentication key path (.p8 file)")
	apnsSandboxKeyID       = flag.String("apns-sandbox-key-id", LookupEnvOrString("APNS_SANDBOX_KEY_ID", ""), "APNS sandbox Key ID from Apple Developer account")
	apnsSandboxTeamID      = flag.String("apns-sandbox-team-id", LookupEnvOrString("APNS_SANDBOX_TEAM_ID", ""), "APNS sandbox Team ID from Apple Developer account")
)

func newLogger() *slog.Logger {
	var opts *slog.HandlerOptions
	if *debug {
		opts = &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, opts))
	return logger
}

func newServiceLogger(service string) *slog.Logger {
	logger := newLogger()
	return logger.With(
		slog.String("service", service),
	)
}

func main() {
	flag.Parse()

	logger := newLogger()
	slog.SetDefault(logger)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	var qf queue.QueueFactory
	if *redisURL == "" {
		slog.Info("Using non-persistent in-memory queue")
		qf = memory.MemoryQueueFactory{}
	} else {
		slog.Info("Using Redis queue at", "address", *redisURL)
		qf = redis.NewQueueFactory(*redisURL)
	}
	s := server.NewServer(*apiAddr, qf)

	if *apnsAuthKeyPath != "" {
		apns, err := apns.NewAPNS(*apnsAuthKeyPath, *apnsKeyID, *apnsTeamID, true, logger)
		if err != nil {
			logger.Error("Failed to initialize APNS", "error", err)
			os.Exit(1)
		}
		if err := s.AddService(apns, *apnsWorkers, services.SquashConfig{}); err != nil {
			slog.Error("Failed to add APNS service", "error", err)
			os.Exit(1)
		}
	}

	if *apnsSandboxAuthKeyPath != "" {
		apns, err := apns.NewAPNS(*apnsSandboxAuthKeyPath, *apnsSandboxKeyID, *apnsSandboxTeamID, false, logger)
		if err != nil {
			logger.Error("Failed to initialize APNS sandbox", "error", err)
			os.Exit(1)
		}
		if err := s.AddService(apns, *apnsWorkers, services.SquashConfig{}); err != nil {
			slog.Error("Failed to add APNS sandbox service", "error", err)
			os.Exit(1)
		}
	}

	if *googleApplicationCredentials != "" {

		fcm, err := fcm.NewFCM(newServiceLogger("fcm"))
		if err != nil {
			slog.Error("Failed to setup FCM service", "error", err)
			os.Exit(1)
		}
		if err := s.AddService(fcm, *fcmWorkers, services.SquashConfig{}); err != nil {
			slog.Error("Failed to add FCM service", "error", err)
			os.Exit(1)
		}
	}

	if *webhookWorkers > 0 {
		wh, err := webhook.NewWebhook(newServiceLogger("webhook"))
		if err != nil {
			slog.Error("Failed to setup Webhook service", "error", err)
			os.Exit(1)
		}
		if err := s.AddService(wh, *webhookWorkers, services.SquashConfig{}); err != nil {
			slog.Error("Failed to add Webhook service", "error", err)
			os.Exit(1)
		}
	}

	if *webPushVAPIDPrivateKey != "" {
		web, err := webpush.NewWebPush(*webPushVAPIDPublicKey, *webPushVAPIDPrivateKey, newServiceLogger("webpush"))
		if err != nil {
			slog.Error("Failed to setup WebPush service", "error", err)
			os.Exit(1)
		}
		if err := s.AddService(web, *webPushWorkers, services.SquashConfig{}); err != nil {
			slog.Error("Failed to add WebPush service", "error", err)
			os.Exit(1)
		}
	}

	if *telegramBotToken != "" {
		tg, err := telegram.NewTelegramService(*telegramBotToken, newServiceLogger("telegram"))
		if err != nil {
			slog.Error("Failed to setup Telegram service", "error", err)
			os.Exit(1)
		}
		if err := s.AddService(tg, *telegramWorkers, services.SquashConfig{
			RateMax: *telegramRateAmount,
			RatePer: time.Second * time.Duration(*telegramRatePer),
		}); err != nil {
			slog.Error("Failed to add Telegram service", "error", err)
			os.Exit(1)
		}
	}

	if *emailHost != "" {
		config := email.EmailConfig{
			EmailHost:     *emailHost,
			EmailPort:     *emailPort,
			TLS:           *emailTLS,
			TLSInsecure:   *emailTLSInsecure,
			Log:           newServiceLogger("email"),
			PlainAuth:     *emailPlainAuth,
			EmailUsername: *emailUsername,
			EmailPassword: *emailPassword,
		}
		email, err := email.NewEmailService(config)
		if err != nil {
			slog.Error("Failed to setup email service", "error", err)
			os.Exit(1)
		}
		if err := s.AddService(email, 1, services.SquashConfig{
			RateMax: *emailRateAmount,
			RatePer: time.Second * time.Duration(*emailRatePer),
		}); err != nil {
			slog.Error("Failed to add email service", "error", err)
			os.Exit(1)
		}
	}

	go func() {
		slog.Info("Serving", "address", *apiAddr)
		err := s.Serve()
		if err != nil {
			slog.Error("Serve failed", "error", err)
			os.Exit(1)
		}
	}()
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Shutdown(ctx)
	slog.Info("Exiting")
}
