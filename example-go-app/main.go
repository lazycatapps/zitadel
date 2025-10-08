package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
)

func init() {
	// 注册 map[string]interface{} 类型以便 gob 可以序列化
	gob.Register(map[string]interface{}{})
}

// export BOXNAME=$(lzc-cli box default)
// export CLIENT_ID=341276400264741379
// export CLIENT_SECRET=KKJdmxMhwDmkYRKQ5lSPZxcjhZqxAAV3jaqeytRrkQKC1SRhiXJiQdhuiOpw0EVq
// export ISSUER_URL=https://zitadel.${BOXNAME}.heiyu.space

var (
	// 从环境变量读取配置
	clientID     = getEnv("CLIENT_ID", "YOUR_CLIENT_ID")
	clientSecret = getEnv("CLIENT_SECRET", "YOUR_CLIENT_SECRET")
	redirectURL  = getEnv("REDIRECT_URL", "http://localhost:3000/auth/callback")
	issuerURL    = getEnv("ISSUER_URL", "https://zitadel.xxx.heiyu.space")

	// Session store
	store *sessions.CookieStore

	// OAuth2 config
	oauth2Config oauth2.Config
	verifier     *oidc.IDTokenVerifier
)

func init() {
	// 初始化 session store
	store = sessions.NewCookieStore([]byte("your-secret-key-change-this-in-production"))
	// 配置 cookie 选项
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 7 天
		HttpOnly: true,
		Secure:   false,                // 本地测试设为 false,生产环境必须设为 true
		SameSite: http.SameSiteLaxMode, // Lax 模式,允许顶级导航携带 cookie
		Domain:   "",                   // 空表示当前域名
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	ctx := context.Background()

	// 初始化 OIDC provider
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		log.Fatalf("Failed to create OIDC provider: %v", err)
	}

	// 配置 OAuth2
	oauth2Config = oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email", "urn:zitadel:iam:user:metadata"},
	}

	// 配置 ID Token 验证器
	verifier = provider.Verifier(&oidc.Config{ClientID: clientID})

	// 路由
	http.HandleFunc("/", handleHome)
	http.HandleFunc("/login", handleLogin)
	http.HandleFunc("/auth/callback", handleCallback)
	http.HandleFunc("/logout", handleLogout)
	http.HandleFunc("/profile", handleProfile)
	http.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		log.Println("=== Test endpoint accessed ===")
		fmt.Fprintf(w, "Test OK! Callback should work.")
	})

	fmt.Println("Server starting on http://localhost:3000")
	fmt.Println("Please update these environment variables:")
	fmt.Println("  CLIENT_ID:", clientID)
	fmt.Println("  CLIENT_SECRET:", clientSecret)
	fmt.Println("  REDIRECT_URL:", redirectURL)
	fmt.Println("  ISSUER_URL:", issuerURL)
	log.Fatal(http.ListenAndServe(":3000", nil))
}

// 首页
func handleHome(w http.ResponseWriter, r *http.Request) {
	log.Println("=== Home page accessed ===")
	log.Println("Request URL:", r.URL.String())
	log.Println("Query params:", r.URL.Query())

	session, err := store.Get(r, "auth-session")
	if err != nil {
		log.Println("Session get error:", err)
	}
	log.Println("Session values:", session.Values)

	userInfo, ok := session.Values["user"].(map[string]interface{})
	log.Println("User logged in:", ok)

	html := `
	<!DOCTYPE html>
	<html>
	<head>
		<title>Zitadel OAuth2 Demo</title>
		<style>
			body { font-family: Arial, sans-serif; max-width: 800px; margin: 50px auto; padding: 20px; }
			.user-info { background: #f0f0f0; padding: 20px; border-radius: 5px; margin: 20px 0; }
			button, a { padding: 10px 20px; background: #007bff; color: white; text-decoration: none;
				border: none; border-radius: 5px; cursor: pointer; display: inline-block; margin: 5px; }
			button:hover, a:hover { background: #0056b3; }
			pre { background: #f5f5f5; padding: 15px; border-radius: 5px; overflow-x: auto; }
		</style>
	</head>
	<body>
		<h1>Zitadel OAuth2 Demo (Golang)</h1>
	`

	if ok && userInfo != nil {
		userJSON, _ := json.MarshalIndent(userInfo, "", "  ")

		// 提取用户信息,如果不存在则使用默认值
		name := "User"
		if n, ok := userInfo["name"].(string); ok && n != "" {
			name = n
		} else if preferred, ok := userInfo["preferred_username"].(string); ok && preferred != "" {
			name = preferred
		}

		email := "N/A"
		if e, ok := userInfo["email"].(string); ok && e != "" {
			email = e
		}

		userID := ""
		if sub, ok := userInfo["sub"].(string); ok {
			userID = sub
		}

		html += fmt.Sprintf(`
		<div class="user-info">
			<h2>Welcome, %s!</h2>
			<p><strong>User ID:</strong> %s</p>
			<p><strong>Email:</strong> %s</p>
			<a href="/profile">View Full Profile</a>
			<a href="/logout">Logout</a>
		</div>
		<h3>User Info (from ID Token):</h3>
		<pre>%s</pre>
		`, name, userID, email, string(userJSON))
	} else {
		html += `
		<p>You are not logged in.</p>
		<a href="/login">Login with Zitadel</a>
		`
	}

	html += `
	</body>
	</html>
	`
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, html)
}

// 登录 - 重定向到 Zitadel
func handleLogin(w http.ResponseWriter, r *http.Request) {
	log.Println("=== Login started ===")

	// 生成 state 防止 CSRF 攻击
	state, err := generateRandomState()
	if err != nil {
		log.Println("Failed to generate state:", err)
		http.Error(w, "Failed to generate state", http.StatusInternalServerError)
		return
	}
	log.Println("Generated state:", state)

	// 保存 state 到 session
	session, err := store.Get(r, "auth-session")
	if err != nil {
		log.Println("Session get error in login:", err)
	}

	session.Values["state"] = state
	log.Println("Session values before save:", session.Values)

	err = session.Save(r, w)
	if err != nil {
		log.Println("Session save error in login:", err)
		http.Error(w, "Failed to save session", http.StatusInternalServerError)
		return
	}
	log.Println("Session saved successfully in login")

	// 重定向到 Zitadel 授权页面
	authURL := oauth2Config.AuthCodeURL(state)
	log.Println("Redirecting to:", authURL)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// 回调处理
func handleCallback(w http.ResponseWriter, r *http.Request) {
	log.Println("=== Callback started ===")
	log.Println("Query params:", r.URL.Query())

	session, _ := store.Get(r, "auth-session")
	log.Println("Session values in callback:", session.Values)

	// 验证 state (暂时注释掉用于调试)
	savedState := session.Values["state"]
	receivedState := r.URL.Query().Get("state")
	log.Println("Saved state:", savedState)
	log.Println("Received state:", receivedState)

	// 暂时跳过 state 验证
	/*
		if receivedState != savedState {
			log.Println("State mismatch!")
			http.Error(w, "Invalid state parameter", http.StatusBadRequest)
			return
		}
	*/
	log.Println("State validation skipped for debugging")

	// 用授权码换取 token
	code := r.URL.Query().Get("code")
	log.Println("Exchanging code:", code)

	oauth2Token, err := oauth2Config.Exchange(r.Context(), code)
	if err != nil {
		log.Println("Token exchange error:", err)
		http.Error(w, "Failed to exchange token: "+err.Error(), http.StatusInternalServerError)
		return
	}
	log.Println("Token exchange successful!")

	// 提取并验证 ID Token
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		log.Println("No id_token in response")
		http.Error(w, "No id_token field in oauth2 token", http.StatusInternalServerError)
		return
	}
	log.Println("ID Token extracted successfully")

	idToken, err := verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		log.Println("ID Token verification error:", err)
		http.Error(w, "Failed to verify ID Token: "+err.Error(), http.StatusInternalServerError)
		return
	}
	log.Println("ID Token verified successfully")

	// 解析用户信息
	var claims map[string]interface{}
	if err := idToken.Claims(&claims); err != nil {
		log.Println("Claims parsing error:", err)
		http.Error(w, "Failed to parse claims: "+err.Error(), http.StatusInternalServerError)
		return
	}
	log.Println("Claims parsed successfully:", claims)

	// 调用 UserInfo 接口获取更完整的用户信息
	log.Println("Fetching user info from UserInfo endpoint...")
	userInfoURL := issuerURL + "/oidc/v1/userinfo"
	req, _ := http.NewRequest("GET", userInfoURL, nil)
	req.Header.Set("Authorization", "Bearer "+oauth2Token.AccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err == nil && resp.StatusCode == 200 {
		var userInfoFromAPI map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&userInfoFromAPI)
		resp.Body.Close()
		log.Println("UserInfo from API:", userInfoFromAPI)

		// 合并 UserInfo 和 ID Token claims
		for k, v := range userInfoFromAPI {
			claims[k] = v
		}
		log.Println("Merged user info:", claims)
	} else {
		log.Println("Failed to fetch UserInfo:", err)
	}

	// 保存用户信息到 session
	session.Values["user"] = claims
	session.Values["access_token"] = oauth2Token.AccessToken
	log.Println("Saving session with user info...")

	err = session.Save(r, w)
	if err != nil {
		log.Println("Session save error:", err)
		http.Error(w, "Failed to save session: "+err.Error(), http.StatusInternalServerError)
		return
	}
	log.Println("Session saved successfully!")
	log.Println("=== Callback completed ===")

	// 重定向回首页
	log.Println("Redirecting to home page...")
	http.Redirect(w, r, "/", http.StatusFound)
}

// 查看完整用户信息
func handleProfile(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "auth-session")
	accessToken, ok := session.Values["access_token"].(string)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	// 调用 UserInfo 接口获取更多信息
	userInfoURL := issuerURL + "/oidc/v1/userinfo"
	req, _ := http.NewRequest("GET", userInfoURL, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Failed to get user info: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var userInfo map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&userInfo)

	// 显示用户信息
	userJSON, _ := json.MarshalIndent(userInfo, "", "  ")
	html := fmt.Sprintf(`
	<!DOCTYPE html>
	<html>
	<head>
		<title>User Profile</title>
		<style>
			body { font-family: Arial, sans-serif; max-width: 800px; margin: 50px auto; padding: 20px; }
			pre { background: #f5f5f5; padding: 15px; border-radius: 5px; overflow-x: auto; }
			a { padding: 10px 20px; background: #007bff; color: white; text-decoration: none;
				border-radius: 5px; display: inline-block; margin: 10px 0; }
		</style>
	</head>
	<body>
		<h1>User Profile</h1>
		<pre>%s</pre>
		<a href="/">Back to Home</a>
	</body>
	</html>
	`, string(userJSON))

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, html)
}

// 登出
func handleLogout(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "auth-session")
	session.Values["user"] = nil
	session.Values["access_token"] = nil
	session.Options.MaxAge = -1
	session.Save(r, w)

	// 重定向到 Zitadel 登出页面
	logoutURL := fmt.Sprintf("%s/oidc/v1/end_session?post_logout_redirect_uri=%s",
		issuerURL, redirectURL)
	http.Redirect(w, r, logoutURL, http.StatusFound)
}

// 生成随机 state
func generateRandomState() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
