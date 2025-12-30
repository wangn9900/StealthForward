package generator

import (
	"fmt"
	"math/rand"
	"time"
)

var companyNames = []string{
	"NexCore Systems", "AetherEdge Solutions", "Vortex Digital", "Zenith Analytics",
	"Lumina Dynamics", "Nebula Cloud Services", "Quantix Global", "Titan Security",
}

var slogans = []string{
	"Empowering the Future of Intelligent Connectivity.",
	"Precision Engineering for a Digital World.",
	"Beyond Security: The Next Generation of Global Infrastructure.",
	"Innovative Solutions for Complex Enterprise Challenges.",
	"Seamless Integration, Unmatched Performance.",
}

var accentColors = []string{
	"#6366f1", // Indigo
	"#ec4899", // Pink
	"#06b6d4", // Cyan
	"#8b5cf6", // Violet
	"#10b981", // Emerald
}

// GenerateMasqueradeHTML 生成一个独一无二的高端伪装页�?
func GenerateMasqueradeHTML() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	company := companyNames[r.Intn(len(companyNames))]
	slogan := slogans[r.Intn(len(slogans))]
	accent := accentColors[r.Intn(len(accentColors))]

	return fmt.Sprintf(`
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s | Official Website</title>
    <link href="https://fonts.googleapis.com/css2?family=Outfit:wght@300;400;600&display=swap" rel="stylesheet">
    <style>
        :root {
            --primary: %s;
            --bg: #0f172a;
            --text: #f8fafc;
        }
        * { margin:0; padding:0; box-sizing: border-box; }
        body {
            font-family: 'Outfit', sans-serif;
            background-color: var(--bg);
            color: var(--text);
            overflow-x: hidden;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
        }
        .container {
            max-width: 1200px;
            padding: 2rem;
            text-align: center;
            z-index: 10;
        }
        .hero-bg {
            position: absolute;
            top: 0; left: 0; width: 100%%; height: 100%%;
            background: radial-gradient(circle at 50%% 50%%, var(--primary) 0%%, transparent 70%%);
            opacity: 0.1;
            filter: blur(80px);
            z-index: 1;
        }
        .glass-card {
            background: rgba(255, 255, 255, 0.03);
            backdrop-filter: blur(12px);
            border: 1px solid rgba(255, 255, 255, 0.1);
            padding: 4rem;
            border-radius: 2rem;
            box-shadow: 0 25px 50px -12px rgba(0,0,0,0.5);
        }
        h1 {
            font-size: 3.5rem;
            font-weight: 600;
            margin-bottom: 1.5rem;
            background: linear-gradient(to right, #fff, var(--primary));
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
        }
        p {
            font-size: 1.2rem;
            color: #94a3b8;
            margin-bottom: 2.5rem;
            line-height: 1.6;
            max-width: 600px;
            margin-inline: auto;
        }
        .btn {
            display: inline-block;
            background: var(--primary);
            color: white;
            padding: 1rem 2.5rem;
            border-radius: 9999px;
            text-decoration: none;
            font-weight: 600;
            transition: all 0.3s ease;
            box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.1), 0 4px 6px -2px rgba(0, 0, 0, 0.05);
        }
        .btn:hover {
            transform: translateY(-2px);
            box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04);
            filter: brightness(1.1);
        }
        .footer {
            margin-top: 4rem;
            font-size: 0.9rem;
            color: #475569;
        }
    </style>
</head>
<body>
    <div class="hero-bg"></div>
    <div class="container">
        <div class="glass-card">
            <h1>%s</h1>
            <p>%s</p>
            <a href="#" class="btn">Explore Solutions</a>
        </div>
        <div class="footer">
            &copy; 2025 %s Global Holdings. All rights reserved.
        </div>
    </div>
</body>
</html>
	`, company, accent, company, slogan, company)
}
