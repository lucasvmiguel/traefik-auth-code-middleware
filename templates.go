package main

import "html/template"

// PageData holds data for rendering templates
type PageData struct {
	Error       string
	Message     string
	RedirectURL string
	Code        string // Preserving code if needed, though usually not for security in URL
}

const loginHTML = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Restricted Access</title>
    <style>
        :root {
            --bg-color: #0f172a;
            --card-bg: #1e293b;
            --text-color: #f1f5f9;
            --accent-color: #3b82f6;
            --accent-hover: #2563eb;
            --error-color: #ef4444;
            --success-color: #22c55e;
        }
        body {
            background-color: var(--bg-color);
            color: var(--text-color);
            font-family: 'Inter', -apple-system, BlinkMacSystemFont, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            height: 100vh;
            margin: 0;
        }
        .container {
            background-color: var(--card-bg);
            padding: 2rem;
            border-radius: 1rem;
            box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.1);
            width: 100%;
            max-width: 400px;
            text-align: center;
        }
        h1 { margin-bottom: 1.5rem; font-size: 1.5rem; }
        p { margin-bottom: 1.5rem; color: #94a3b8; }
        input {
            width: 100%;
            padding: 0.75rem;
            border-radius: 0.5rem;
            border: 1px solid #475569;
            background-color: #334155;
            color: white;
            margin-bottom: 1rem;
            box-sizing: border-box;
            font-size: 1rem;
            text-align: center;
            letter-spacing: 2px;
        }
        button {
            width: 100%;
            padding: 0.75rem;
            border-radius: 0.5rem;
            border: none;
            background-color: var(--accent-color);
            color: white;
            font-weight: 600;
            cursor: pointer;
            transition: background-color 0.2s;
            font-size: 1rem;
        }
        button:hover { background-color: var(--accent-hover); }
        .message { margin-top: 1rem; font-size: 0.875rem; min-height: 1.25rem; }
        .error { color: var(--error-color); }
        .success { color: var(--success-color); }
        .footer { margin-top: 2rem; font-size: 0.75rem; color: #64748b; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🔐 Protected Resource</h1>
        
        <form action="/auth/request-code" method="POST">
            <input type="hidden" name="redirect_url" value="{{.RedirectURL}}">
            <p>This resource is protected. Please request an access code to continue.</p>
            {{if .Error}}<p class="message error">{{.Error}}</p>{{end}}
            {{if .Message}}<p class="message success">{{.Message}}</p>{{end}}
            <button type="submit">Send Access Code</button>
        </form>

        <div class="footer">Traefik Auth Middleware</div>
    </div>
</body>
</html>
`

const verifyHTML = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Verify Access</title>
    <style>
        :root {
            --bg-color: #0f172a;
            --card-bg: #1e293b;
            --text-color: #f1f5f9;
            --accent-color: #3b82f6;
            --accent-hover: #2563eb;
            --error-color: #ef4444;
            --success-color: #22c55e;
        }
        body {
            background-color: var(--bg-color);
            color: var(--text-color);
            font-family: 'Inter', -apple-system, BlinkMacSystemFont, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            height: 100vh;
            margin: 0;
        }
        .container {
            background-color: var(--card-bg);
            padding: 2rem;
            border-radius: 1rem;
            box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.1);
            width: 100%;
            max-width: 400px;
            text-align: center;
        }
        h1 { margin-bottom: 1.5rem; font-size: 1.5rem; }
        p { margin-bottom: 1.5rem; color: #94a3b8; }
        input {
            width: 100%;
            padding: 0.75rem;
            border-radius: 0.5rem;
            border: 1px solid #475569;
            background-color: #334155;
            color: white;
            margin-bottom: 1rem;
            box-sizing: border-box;
            font-size: 1rem;
            text-align: center;
            letter-spacing: 2px;
        }
        button {
            width: 100%;
            padding: 0.75rem;
            border-radius: 0.5rem;
            border: none;
            background-color: var(--accent-color);
            color: white;
            font-weight: 600;
            cursor: pointer;
            transition: background-color 0.2s;
            font-size: 1rem;
        }
        button:hover { background-color: var(--accent-hover); }
        .message { margin-top: 1rem; font-size: 0.875rem; min-height: 1.25rem; }
        .error { color: var(--error-color); }
        .success { color: var(--success-color); }
        .footer { margin-top: 2rem; font-size: 0.75rem; color: #64748b; }
        .resend { margin-top: 10px; }
        .resend a { color: #64748b; font-size: 0.8rem; text-decoration: none; }
        .resend a:hover { text-decoration: underline; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🔐 Verify Code</h1>
        
        <form action="/auth/verify-code" method="POST">
            <input type="hidden" name="redirect_url" value="{{.RedirectURL}}">
            <p>A code has been sent to your configured notification channel.</p>
            
            <input type="text" name="code" placeholder="123456" maxlength="6" autofocus required autocomplete="off">
            
            {{if .Error}}<p class="message error">{{.Error}}</p>{{end}}
            {{if .Message}}<p class="message success">{{.Message}}</p>{{end}}
            
            <button type="submit">Verify Code</button>
        </form>
        
        <div class="resend">
             <form action="/auth/login" method="GET" style="display:inline;">
                 <input type="hidden" name="redirect_url" value="{{.RedirectURL}}">
                <button type="submit" style="background:none; border:none; color:#64748b; font-size:0.8rem; padding:0; width:auto; cursor:pointer; text-decoration:underline;">Resend Code</button>
            </form>
        </div>

        <div class="footer">Traefik Auth Middleware</div>
    </div>
</body>
</html>
`

var (
	loginTmpl  = template.Must(template.New("login").Parse(loginHTML))
	verifyTmpl = template.Must(template.New("verify").Parse(verifyHTML))
)
