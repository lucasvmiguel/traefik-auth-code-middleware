# Traefik Auth Code Middleware

A lightweight, zero-trust authentication middleware for Traefik.  
This middleware protects your internal services by requiring a temporary access code sent via **Telegram** or **Discord**.

It is designed for personal use to expose internal services securely without a complex identity provider.

## Features

- **No Database**: Fully in-memory standard library implementation.
- **Short-lived Codes**: Codes are valid for a short time (default 5m) and bound to the request IP.
- **Notifications**: Supports Telegram or Discord (one active at a time).
- **Brute-force Protection**: Delays verification responses to slow down guessing attempts.
- **Traefik Integration**: Works as a standard ForwardAuth middleware.

## Configuration

Configuration can be done via command-line flags or environment variables.

| Flag                    | Environment Variable  | Description                       | Default                 |
| :---------------------- | :-------------------- | :-------------------------------- | :---------------------- |
| `--port`                | `PORT`                | HTTP Port to listen on            | `8080`                  |
| `--telegram-bot-token`  | `TELEGRAM_BOT_TOKEN`  | Telegram Bot Token                | (Required for Telegram) |
| `--telegram-chat-id`    | `TELEGRAM_CHAT_ID`    | Telegram Chat ID to send codes to | (Required for Telegram) |
| `--discord-webhook-url` | `DISCORD_WEBHOOK_URL` | Discord Webhook URL               | (Required for Discord)  |
| `--code-expiration`     | `CODE_EXPIRATION`     | Duration code remains valid       | `5m`                    |
| `--session-duration`    | `SESSION_DURATION`    | Duration of authenticated session | `24h`                   |
| `--code-length`         | `CODE_LENGTH`         | Length of the generated code      | `6`                     |
| `--cookie-name`         | `COOKIE_NAME`         | Name of the session cookie        | `traefik_auth_code`     |

**Note**: You must provide either Telegram OR Discord credentials.

## Installation & Traefik Configuration

### 1. Deploy the Middleware

Add the middleware to your `docker-compose.yml`:

```yaml
services:
  auth-middleware:
    image: ghcr.io/lucasvieira/traefik-auth-code-middleware:latest
    container_name: auth-middleware
    restart: unless-stopped
    environment:
      - TELEGRAM_BOT_TOKEN=your_bot_token
      - TELEGRAM_CHAT_ID=your_chat_id
      # - DISCORD_WEBHOOK_URL=your_webhook_url
      - SESSION_DURATION=720h # 30 days
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.auth-server.rule=PathPrefix(`/login`) || PathPrefix(`/request-code`) || PathPrefix(`/verify-code`)"
      - "traefik.http.routers.auth-server.priority=100"
      - "traefik.http.routers.auth-server.tls=true"
      - "traefik.http.routers.auth-server.service=auth-middleware"
      - "traefik.http.services.auth-middleware.loadbalancer.server.port=8080"
```

### 2. Configure the Traefik Middleware

You need to define a `forwardAuth` middleware in Traefik that points to this service.

#### Using Docker Labels (on the `traefik` container or any service)

Add this label to define the middleware globally or on a specific router:

```yaml
# In your dynamic config or labels:
traefik.http.middlewares.my-auth.forwardauth.address: "http://auth-middleware:8080"
traefik.http.middlewares.my-auth.forwardauth.trustForwardHeader: true
# Remove authResponseHeaders if previously set to handle redirects,
# or ensure it doesn't conflict. The middleware handles the response body.
```

### 3. Apply the Middleware to Your Services

Add the label to the services you want to protect:

```yaml
services:
  whoami:
    image: traefik/whoami
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.whoami.rule=Host(`whoami.yourdomain.com`)"
      - "traefik.http.routers.whoami.middlewares=my-auth" # Apply the middleware here
```

## How it Works

1. **User Request**: User visits a protected domain (e.g., `whoami.yourdomain.com`).
2. **Traefik ForwardAuth**: Traefik intercepts the request and forwards headers to `auth-middleware`.
3. **Session Check**:
   - **Valid Session**: Middleware returns `200 OK`, and Traefik lets the request through.
   - **No Session**: Middleware returns `401 Unauthorized` and renders a **Login Page** (HTML Form).
4. **Request Code**:
   - User clicks "Send Access Code".
   - Browser POSTs to `/request-code`.
   - Middleware generates a 6-digit code, securely stores it (linked to IP), and sends it to your configured notification channel (Telegram/Discord).
   - Middleware renders the **Verify Page**.
5. **Verify Code**:
   - User enters the code.
   - Browser POSTs to `/verify-code`.
   - Middleware verifies the code matches the IP and hasn't expired.
   - If valid, a secure HTTP-only session cookie is set.
   - User is redirected back to the original URL or shown a success message.

## Development

```bash
# Run locally
go run . --port 8081 --telegram-bot-token "..." --telegram-chat-id "..."
```

## Testing

Run unit and integration tests using standard Go tooling:

```bash
go test ./...
```
