# When push comes to shove...

[![Go Report Card](https://goreportcard.com/badge/github.com/mattstrayer/shove)](https://goreportcard.com/report/github.com/mattstrayer/shove) [![Written in Emacs](https://pennersr.github.io/img/emacs-badge.svg)](https://www.gnu.org/software/emacs/) [![Pipeline Status](https://github.com/mattstrayer/shove/badges/master/pipeline.svg)](https://github.com/mattstrayer/shove/-/pipelines)

## Background

This is the replacement for [Pulsus](https://github.com/pennersr/pulsus) which has been steadily serving up to 100M push notifications. But, given that it was still using the binary APNS protocol it was due for an upgrade.

## Overview

Design:
- Asynchronous: a push client can just fire & forget.
- Multiple workers per push service.
- Less moving parts: when using Redis, you can push directly to the queue, bypassing the need for the Shove server to be up and running.

Supported push services:
- APNS
- Email: supports automatic creation of email digests in case the rate limit
  is exceeded
- FCM
- Telegram: supports squashing multiple messages into one in case the rate limit
  is exceeded
- Webhook: issue arbitrary webhook posts
- Web Push

Features:
- Feedback: asynchronously receive information on invalid device tokens.
- Queueing: both in-memory and persistent via Redis.
- Exponential back-off in case of failure.
- Prometheus support.
- Squashing of messages in case rate limits are exceeded.


## Why?

- https://github.com/appleboy/gorush/issues/386#issuecomment-479191179

- https://github.com/mercari/gaurun/issues/115


## Usage

### Running

Usage:

    $ shove -h
    Usage of ./shove:
      -api-addr string
            API address to listen to (default ":8322")
      -worker-only
            Run in worker-only mode (no HTTP server)
      -apns-certificate-path string
            APNS certificate path
      -apns-sandbox-certificate-path string
            APNS sandbox certificate path
      -apns-workers int
            The number of workers pushing APNS messages (default 4)
      -email-host string
            Email host
      -email-port int
            Email port (default 25)
      -email-rate-amount int
            Email max. rate (amount)
      -email-rate-per int
            Email max. rate (per seconds)
      -email-tls
            Use TLS
      -email-tls-insecure
            Skip TLS verification
      -fcm-api-key string
            FCM API key
      -fcm-workers int
            The number of workers pushing FCM messages (default 4)
      -redis-host string
            Redis host
      -redis-port string
            Redis port (default "6379")
      -redis-password string
            Redis password
      -redis-db string
            Redis database number (default "0")
      -telegram-bot-token string
            Telegram bot token
      -telegram-rate-amount int
            Telegram max. rate (amount)
      -telegram-rate-per int
            Telegram max. rate (per seconds)
      -telegram-workers int
            The number of workers pushing Telegram messages (default 2)
      -webhook-workers int
            The number of workers pushing Webhook messages
      -webpush-vapid-private-key string
            VAPID public key
      -webpush-vapid-public-key string
            VAPID public key
      -webpush-workers int
            The number of workers pushing Web messages (default 8)


Start the server:

    $ shove \
        -api-addr localhost:8322 \
        -redis-host localhost \
        -fcm-api-key $FCM_API_KEY \
        -apns-certificate-path /etc/shove/apns/production/bundle.pem -apns-sandbox-certificate-path /etc/shove/apns/sandbox/bundle.pem \
        -webpush-vapid-public-key=$VAPID_PUBLIC_KEY -webpush-vapid-private-key=$VAPID_PRIVATE_KEY \
        -telegram-bot-token $TELEGRAM_BOT_TOKEN


### APNS

Push an APNS notification:

```bash
$ curl  -i  --data '{"service": "apns", "headers": {"apns-priority": 10, "apns-topic": "com.shove.app"}, "payload": {"aps": { "alert": "hi"}}, "token": "81b8ecff8cb6d22154404d43b9aeaaf6219dfbef2abb2fe313f3725f4505cb47"}' http://localhost:8322/api/push/apns
```

APNS configuration options:

**Option 1: File path** (for local development or when mounting files)
- `-apns-auth-key-path` or `APNS_AUTH_KEY_PATH`: Path to the APNS authentication key file (.p8)
- `-apns-key-id` or `APNS_KEY_ID`: Key ID from Apple Developer account
- `-apns-team-id` or `APNS_TEAM_ID`: Team ID from Apple Developer account
- `-apns-workers` or `APNS_WORKERS`: The number of workers pushing APNS messages (default 4)

**Option 2: Base64-encoded key** (for cloud deployments like Digital Ocean App Platform)
- `-apns-auth-key` or `APNS_AUTH_KEY`: Base64-encoded content of the APNS authentication key file (.p8)
- `-apns-key-id` or `APNS_KEY_ID`: Key ID from Apple Developer account
- `-apns-team-id` or `APNS_TEAM_ID`: Team ID from Apple Developer account
- `-apns-workers` or `APNS_WORKERS`: The number of workers pushing APNS messages (default 4)

For sandbox environment:
- `-apns-sandbox-auth-key-path` or `APNS_SANDBOX_AUTH_KEY_PATH`: Path to the APNS sandbox authentication key file (.p8)
- `-apns-sandbox-auth-key` or `APNS_SANDBOX_AUTH_KEY`: Base64-encoded content of the APNS sandbox authentication key file (.p8)
- `-apns-sandbox-key-id` or `APNS_SANDBOX_KEY_ID`: Sandbox Key ID from Apple Developer account
- `-apns-sandbox-team-id` or `APNS_SANDBOX_TEAM_ID`: Sandbox Team ID from Apple Developer account

**Getting base64-encoded key content:**

To convert your `.p8` file to base64 for use with `APNS_AUTH_KEY`:

```bash
# Linux
base64 -i AuthKey_XXXXX.p8 | tr -d '\n'

# macOS
base64 -i AuthKey_XXXXX.p8 | tr -d '\n'
```

Example using file path:
```bash
$ shove \
  -apns-auth-key-path /etc/shove/apns/production/AuthKey_ABCD1234.p8 \
  -apns-key-id ABCD1234 \
  -apns-team-id XYZ1234567 \
  -apns-sandbox-auth-key-path /etc/shove/apns/sandbox/AuthKey_EFGH5678.p8 \
  -apns-sandbox-key-id EFGH5678 \
  -apns-sandbox-team-id XYZ1234567 \
  -apns-workers 4
```

Example using environment variables with base64-encoded key:
```bash
export APNS_AUTH_KEY=$(base64 -i AuthKey_ABCD1234.p8 | tr -d '\n')
export APNS_KEY_ID=ABCD1234
export APNS_TEAM_ID=XYZ1234567
export APNS_WORKERS=4

$ shove
```


A successful push results in:

    HTTP/1.1 202 Accepted
    Date: Tue, 07 May 2019 19:00:15 GMT
    Content-Length: 2
    Content-Type: text/plain; charset=utf-8

    OK


### FCM

FCM configuration options:

**Option 1: File path** (for local development or when mounting files)
- `-google-application-credentials` or `GOOGLE_APPLICATION_CREDENTIALS`: Path to the Google application credentials JSON file
- `-fcm-workers` or `FCM_WORKERS`: The number of workers pushing FCM messages (default 4)

**Option 2: Base64-encoded JSON** (for cloud deployments like Digital Ocean App Platform)
- `-google-application-credentials-json` or `GOOGLE_APPLICATION_CREDENTIALS_JSON`: Base64-encoded content of the Google application credentials JSON file
- `-fcm-workers` or `FCM_WORKERS`: The number of workers pushing FCM messages (default 4)

**Getting base64-encoded credentials:**

To convert your Google credentials JSON file to base64:

```bash
# Linux
base64 -i google-application-credentials.json | tr -d '\n'

# macOS
base64 -i google-application-credentials.json | tr -d '\n'
```

Push an FCM notification:

    $ curl  -i  --data '{"to": "feE8R6apOdA:AA91PbGHMX5HUoB-tbcqBO_e75NbiOc2AiFbGL3rrYtc99Z5ejbGmCCvOhKW5liqfOzRGOXxto5l7y6b_0dCc-AQ2_bXOcDkcPZgsXGbZvmEjaZA72DfVkZ2pfRrcpcc_9IiiRT5NYC", "notification": {"title": "Hello"}}' http://localhost:8322/api/push/fcm


### Webhook

Push a Webhook call, containing arbitrary body content:

    $ curl  -i  --data '{"url": "http://localhost:8000/api/webhook", "headers": {"foo": "bar"}, "body": "Hello world!"}' http://localhost:8322/api/push/webhook

Or, post JSON:

    $ curl  -i  --data '{"url": "http://localhost:8000/api/webhook", "headers": {"foo": "bar"}, "data": {"hello": "world!"}}' http://localhost:8322/api/push/webhook


### WebPush

Push a WebPush notification:

    $ curl  -i  --data '{"subscription": {"endpoint":"https://updates.push.services.mozilla.com/wpush/v2/gAAAAAc4BA....UrjGlg","keys":{"auth":"Hbj3ap...al9ew","p256dh":"BeKdTC3...KLGBJlgF"}}, "headers": {"ttl": 3600, "urgency": "high"}, "token": "use-this-for-feedback-instead-of-subscription", "payload": {"hello":"world"}}' http://localhost:8322/api/push/webpush

The subscription (serialized as a JSON string) is used for receiving
feedback. Alternatively, you can specify an optional `token` parameter as done
in the example above.


### Telegram

Push a Telegram notification:

    $ curl  -i  --data '{"method": "sendMessage", "payload": {"chat_id": "12345678", "text": "Hello!"}}' http://localhost:8322/api/push/telegram

Note that the Telegram Bot API documents `chat_id` as "Integer or String" --
Shove requires strings to be passed. For users that disconnected from your bot
the chat ID will be communicated back through the feedback mechanism. Here, the
token will equal the unreachable chat ID.


### Receive Feedback

Outdated/invalid device tokens (from APNS and FCM) are communicated back through the feedback system. When Redis is configured, feedback is persisted to the `shove:feedback` Redis key and survives server restarts. Without Redis, feedback is stored in-memory and lost on restart.

#### HTTP API

**Pop feedback (retrieve and remove):**

    $ curl -X POST 'http://localhost:8322/api/feedback?limit=100'

    {
      "feedback": [
        {
          "service": "apns-sandbox",
          "token": "881becff86cbd221544044d3b9aeaaf6314dfbef2abb2fe313f3725f4505cb47",
          "reason": "invalid",
          "timestamp": 1701705600
        }
      ]
    }

**Peek feedback (retrieve without removing):**

    $ curl 'http://localhost:8322/api/feedback/peek?limit=100'

    {
      "feedback": [...],
      "total": 42
    }

#### Direct Redis Access (for cron jobs)

When using Redis, your cron job or external service can consume feedback directly from the `shove:feedback` Redis list:

```python
import redis
import json

r = redis.Redis(host='localhost', port=6379, db=0)

# Pop oldest entries (FIFO)
while True:
    item = r.rpop('shove:feedback')
    if not item:
        break
    feedback = json.loads(item)
    # Process: remove token from your database
    print(f"Invalid token for {feedback['service']}: {feedback['token']}")
```

Each feedback entry contains:
- `service`: The service ID (e.g., `apns`, `apns-sandbox`, `fcm`)
- `token`: The invalid/replaced device token
- `replacement_token`: (optional) New token to use instead
- `reason`: Either `invalid` or `replaced`
- `timestamp`: Unix timestamp when the feedback was recorded


### Email

In order to keep your SMTP server safe from being blacklisted, the email service
supports rate limitting. When the rate is exceeded, multiple mails are
automatically digested.

    $ shove \
        -email-host localhost \
        -email-port 1025 \
        -api-addr localhost:8322 \
        -email-rate-amount 3 \
        -email-rate-per 10 \
        -redis-host localhost

Push an email:

	$ curl -i -X POST --data @./scripts/email.json http://localhost:8322/api/push/email

If you send too many emails, you'll notice that they are digested, and at a
later time, one digest mail is being sent:

    2021/03/23 21:15:57 Using Redis queue host=localhost port=6379 db=0
    2021/03/23 21:15:57 Initializing Email service
    2021/03/23 21:15:57 Serving on localhost:8322
    2021/03/23 21:15:57 Shove server started
    2021/03/23 21:15:57 email: Worker started
    2021/03/23 21:15:57 email: Digester started
    2021/03/23 21:15:58 email: Sending email
    2021/03/23 21:15:59 email: Sending email
    2021/03/23 21:15:59 email: Sending email
    2021/03/23 21:16:00 email: Rate to john@doe.org exceeded, email digested
    2021/03/23 21:16:12 email: Rate to john@doe.org exceeded, email digested
    2021/03/23 21:16:18 email: Sending digest email


### Redis Queues

Shove is being used to push a high volume of notifications in a production
environment, consisting of various microservices interacting together. In such a
scenario, it is important that the various services are not too tightly coupled
to one another.  For that purpose, Shove offers the ability to post
notifications directly to a Redis queue.

Posting directly to the Redis queue, instead of using the HTTP service
endpoints, has the advantage that you can take Shove offline without disturbing
the operation of the clients pushing the notifications.

#### Redis Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_HOST` | (required) | Redis host |
| `REDIS_PORT` | `6379` | Redis port |
| `REDIS_PASSWORD` | (empty) | Redis password |
| `REDIS_DB` | `0` | Redis database number |

Example:
```bash
REDIS_HOST=10.116.0.3
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0
```

If `REDIS_HOST` is not set, Shove falls back to a non-persistent in-memory queue.

#### Worker-Only Mode

For deployments where messages are pushed directly to Redis queues and no HTTP API is needed, you can run Shove in worker-only mode to save resources:

```bash
# Via flag
shove -worker-only -redis-host localhost ...

# Via environment variable
WORKER_ONLY=true shove -redis-host localhost ...
```

In worker-only mode:
- No HTTP server is started (saves memory and CPU)
- Workers still process messages from Redis queues
- Feedback is still stored in Redis (`shove:feedback`)
- Consume feedback directly from Redis using your cron job

Shove intentionally tries to make as little assumptions on the notification
payloads being pushed, as they are mostly handed over as is to the upstream
services. So, when using Shove this way, the client is responsible for handing
over a raw payload. Here's an example:


    package main

    import (
    	"encoding/json"
    	"github.com/mattstrayer/shove/pkg/shove"
    	"log"
    	"os"
    )

    type FCMNotification struct {
    	To       string            `json:"to"`
    	Data     map[string]string `json:"data,omitempty"`
    }

    func main() {
    	redisURL := os.Getenv("REDIS_URL")
    	if redisURL == "" {
    		redisURL = "redis://localhost:6379"
    	}
    	client := shove.NewRedisClient(redisURL)

    	notification := FCMNotification{
    		To:   "token....",
    		Data: map[string]string{},
    	}

    	raw, err := json.Marshal(notification)
    	if err != nil {
    		log.Fatal(err)
    	}
    	err = client.PushRaw("fcm", raw)
    	if err != nil {
    		log.Fatal(err)
    	}
    }


## Status

Used in production, over at:

- [Drakdoo: Indicator based signals & alerts](https://www.drakdoo.com): 365.251.428 alerts fired and counting.
