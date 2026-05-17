package build

import "strings"

func renderHTML(block string) string {
	block = strings.ReplaceAll(block, "<re", "<div class='card'")
	block = strings.ReplaceAll(block, "</re>", "</div>")

	block = strings.ReplaceAll(block, "<ta", "<div class='section'")
	block = strings.ReplaceAll(block, "</ta>", "</div>")

	block = strings.ReplaceAll(block, "<button", "<button class='btn'")

	return `
<!DOCTYPE html>
<html lang="pt-br">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Flapjack Panel</title>

<script src="https://unpkg.com/htmx.org@1.9.10"></script>

<style>
:root {
	--bg: #0a0d12;
	--card: rgba(255,255,255,0.04);
	--stroke: rgba(255,255,255,0.08);
	--text: #e6edf3;
	--muted: #8b949e;
	--accent: #7c3aed;
	--green: #22c55e;
	--red: #ef4444;
}

* { box-sizing: border-box; }

body {
	margin: 0;
	background: radial-gradient(circle at top, #111827, var(--bg));
	color: var(--text);
	font-family: ui-sans-serif, system-ui;
	padding: 28px;
}

/* topo */
.topbar {
	padding: 12px 16px;
	border-radius: 14px;
	background: rgba(124,58,237,0.12);
	border: 1px solid rgba(124,58,237,0.25);
	margin-bottom: 18px;
	font-weight: 600;
}

/* grid */
.container {
	display: grid;
	gap: 16px;
}

/* cards */
.card {
	background: var(--card);
	border: 1px solid var(--stroke);
	backdrop-filter: blur(10px);
	padding: 16px;
	border-radius: 16px;
	transition: 0.2s ease;
}

.card:hover {
	transform: translateY(-2px);
	border-color: rgba(124,58,237,0.4);
}

/* sections */
.section {
	margin-top: 10px;
	padding: 12px;
	border-left: 3px solid var(--accent);
	background: rgba(0,0,0,0.25);
	border-radius: 10px;
}

/* server line */
.server {
	display: flex;
	justify-content: space-between;
	align-items: center;
	padding: 10px 0;
	border-bottom: 1px solid rgba(255,255,255,0.05);
}

.server:last-child { border-bottom: none; }

/* status */
.status {
	font-size: 12px;
	padding: 3px 8px;
	border-radius: 999px;
	margin-left: 8px;
}

.online {
	background: rgba(34,197,94,0.15);
	color: var(--green);
}

.offline {
	background: rgba(239,68,68,0.15);
	color: var(--red);
}

/* buttons */
.btn {
	background: linear-gradient(135deg, #7c3aed, #5b21b6);
	border: none;
	padding: 6px 12px;
	border-radius: 10px;
	color: white;
	cursor: pointer;
	font-size: 12px;
	transition: 0.2s;
}

.btn:hover {
	transform: scale(1.05);
}

/* HTMX animations */
.htmx-swapping {
	opacity: 0;
	transform: translateY(6px);
	transition: all 0.2s ease;
}

.htmx-settling {
	opacity: 1;
	transform: translateY(0);
	transition: all 0.2s ease;
}

/* loading effect */
.htmx-request {
	position: relative;
	overflow: hidden;
}

.htmx-request::after {
	content: "";
	position: absolute;
	inset: 0;
	background: linear-gradient(90deg, transparent, rgba(255,255,255,0.06), transparent);
	animation: shimmer 1.1s infinite;
}

@keyframes shimmer {
	0% { transform: translateX(-100%); }
	100% { transform: translateX(100%); }
}

/* toast */
#toast {
	position: fixed;
	bottom: 18px;
	right: 18px;
	background: rgba(124,58,237,0.9);
	color: white;
	padding: 10px 14px;
	border-radius: 12px;
	font-size: 13px;
	display: none;
	backdrop-filter: blur(10px);
}
</style>
</head>

<body>

<div class="topbar">
⚡ Flapjack Dashboard • live panel
</div>

<div class="container">
` + block + `
</div>

<div id="toast"></div>

<script>
document.body.addEventListener('htmx:afterRequest', function() {
	showToast("✔ atualizado");
});

document.body.addEventListener('htmx:responseError', function() {
	showToast("❌ erro na requisição");
});

function showToast(msg) {
	const t = document.getElementById("toast");
	if (!t) return;

	t.innerText = msg;
	t.style.display = "block";

	setTimeout(() => {
		t.style.display = "none";
	}, 1800);
}
</script>

</body>
</html>
`
}
