package main

import "html/template"

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
        .hidden { display: none; }
        .message { margin-top: 1rem; font-size: 0.875rem; min-height: 1.25rem; }
        .error { color: var(--error-color); }
        .success { color: var(--success-color); }
        .footer { margin-top: 2rem; font-size: 0.75rem; color: #64748b; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🔐 Protected Resource</h1>
        
        <div id="step-request">
            <p>This resource is protected. Please request an access code to continue.</p>
            <button onclick="requestCode()" id="btn-request">Send Access Code</button>
        </div>

        <div id="step-verify" class="hidden">
            <p>A code has been sent to your configured notification channel.</p>
            <input type="text" id="code-input" placeholder="123456" maxlength="6" autofocus>
            <button onclick="verifyCode()" id="btn-verify">Verify Code</button>
            <div style="margin-top: 10px;">
                <a href="#" onclick="resetFlow()" style="color: #64748b; font-size: 0.8rem; text-decoration: none;">Resend Code</a>
            </div>
        </div>

        <div id="message" class="message"></div>
        <div class="footer">Traefik Auth Middleware</div>
    </div>

    <script>
        const API_BASE = window.location.origin; // Or relative paths

        async function requestCode() {
            const btn = document.getElementById('btn-request');
            btn.disabled = true;
            btn.innerText = "Sending...";
            
            try {
                const res = await fetch('/auth/request-code', { method: 'POST' });
                const data = await res.json();
                
                if (res.ok) {
                    document.getElementById('step-request').classList.add('hidden');
                    document.getElementById('step-verify').classList.remove('hidden');
                    showMessage("Code sent!", "success");
                    document.getElementById('code-input').focus();
                } else {
                    showMessage(data.error || "Failed to send code", "error");
                    btn.disabled = false;
                    btn.innerText = "Send Access Code";
                }
            } catch (e) {
                showMessage("Network error", "error");
                btn.disabled = false;
                btn.innerText = "Send Access Code";
            }
        }

        async function verifyCode() {
            const code = document.getElementById('code-input').value;
            const btn = document.getElementById('btn-verify');
            if (!code) return;

            btn.disabled = true;
            btn.innerText = "Verifying...";
            
            try {
                const res = await fetch('/auth/verify-code', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ code })
                });
                
                if (res.ok) {
                    showMessage("Access Granted! Redirecting...", "success");
                    // Reload to let the middleware pass the request essentially
                    setTimeout(() => {
                        const urlParams = new URLSearchParams(window.location.search);
                        const redirect = urlParams.get('rd'); // rd param from traefik forward auth usually? 
                        // Actually, Traefik Forward Auth might not pass the original URL easily unless configured.
                        // Standard behavior: if we set a cookie, subsequent requests to the original URL will pass.
                        // We should instruct user to reload or redirect to root /.
                        window.location.href = redirect || "/"; 
                    }, 1000);
                } else {
                    const data = await res.json();
                    showMessage(data.error || "Invalid code", "error");
                    btn.disabled = false;
                    btn.innerText = "Verify Code";
                    document.getElementById('code-input').value = "";
                }
            } catch (e) {
                showMessage("Verification failed", "error");
                btn.disabled = false;
                btn.innerText = "Verify Code";
            }
        }

        function showMessage(msg, type) {
            const el = document.getElementById('message');
            el.innerText = msg;
            el.className = "message " + type;
        }

        function resetFlow() {
            document.getElementById('step-request').classList.remove('hidden');
            document.getElementById('step-verify').classList.add('hidden');
            showMessage("", "");
            document.getElementById('btn-request').disabled = false;
            document.getElementById('btn-request').innerText = "Send Access Code";
        }

        // Handle Enter key
        document.getElementById('code-input').addEventListener('keypress', function (e) {
            if (e.key === 'Enter') {
                verifyCode();
            }
        });
    </script>
</body>
</html>
`

var loginTmpl = template.Must(template.New("login").Parse(loginHTML))
