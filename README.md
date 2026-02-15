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

Configuration is done via environment variables:

| Variable              | Description                           | Default                 |
| --------------------- | ------------------------------------- | ----------------------- |
| `PORT`                | HTTP Port to listen on                | `8080`                  |
| `TELEGRAM_BOT_TOKEN`  | Telegram Bot Token                    | (Required for Telegram) |
| `TELEGRAM_CHAT_ID`    | Telegram Chat ID to send codes to     | (Required for Telegram) |
| `DISCORD_WEBHOOK_URL` | Discord Webhook URL                   | (Required for Discord)  |
| `CODE_EXPIRATION`     | Duration a code is valid (e.g., `5m`) | `5m`                    |
| `SESSION_DURATION`    | Duration the session cookie is valid  | `24h`                   |
| `COOKIE_NAME`         | Name of the session cookie            | `traefik_auth_code`     |

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
      # 1. Router for the Auth UI pages (/auth/*)
      - "traefik.http.routers.auth-server.rule=PathPrefix(`/auth/`)"
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

1. User visits `whoami.yourdomain.com`.
2. Traefik sends the request to `auth-middleware`.
3. Middleware checks for a valid cookie.
   - **Valid**: Returns 200 OK. Traefik allows the request to pass.
   - **Invalid**: Returns 401 Unauthorized and serves the login HTML page directly.
4. User sees the login page on `whoami.yourdomain.com`.
5. User requests a code (AJAX POST to `/auth/request-code`).
6. Middleware generates a code and sends it to Telegram/Discord.
7. User enters the code (AJAX POST to `/auth/verify-code`).
8. Middleware verifies code, sets a session cookie, and returns success.
9. JavaScript on the page reloads the window.
10. Traefik sees the valid cookie and allows access.

## Development

```bash
# Run locally
go run .

# Build Docker image
docker build -t traefik-auth-code-middleware .
```

## Testing

If you encounter issues running tests locally (e.g., `dyld: missing LC_UUID` on macOS), it is recommended to run tests inside Docker to ensure a consistent environment:

```bash
docker build -t test-middleware-with-tests .
```

This will run all unit and integration tests during the build process.
