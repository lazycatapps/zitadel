# Zitadel OAuth2 Demo (Golang)

这是一个使用 Golang 集成 Zitadel OAuth2/OIDC 认证的完整示例。

## 功能特性

- ✅ OAuth2 Authorization Code Flow
- ✅ OIDC ID Token 验证
- ✅ 获取用户信息 (UserInfo endpoint)
- ✅ Session 管理
- ✅ 登出功能
- ✅ CSRF 防护 (state parameter)

## 前置要求

1. 在 Zitadel 中创建好应用,获取 Client ID 和 Client Secret
2. 在 Zitadel 应用配置中添加回调地址: `http://localhost:3000/auth/callback`
3. 安装 Go 1.21+

## 快速开始

### 1. 安装依赖

```bash
cd example-go-app
go mod tidy
```

### 2. 配置环境变量

复制 `.env.example` 为 `.env` 并填入你的配置:

```bash
cp .env.example .env
```

编辑 `.env` 文件:

```
CLIENT_ID=你的_CLIENT_ID
CLIENT_SECRET=你的_CLIENT_SECRET
REDIRECT_URL=http://localhost:3000/auth/callback
ISSUER_URL=https://zitadel.${BOXNAME}.heiyu.space
```

### 3. 运行应用

```bash
# 方式 1: 直接设置环境变量
export CLIENT_ID="你的_CLIENT_ID"
export CLIENT_SECRET="你的_CLIENT_SECRET"
go run main.go

# 方式 2: 使用 .env 文件 (需要安装 godotenv)
go run main.go
```

### 4. 访问应用

打开浏览器访问: http://localhost:3000

## 使用流程

1. **首页** - 显示未登录状态,点击 "Login with Zitadel"
2. **登录** - 跳转到 Zitadel 登录页面,输入用户名密码
3. **授权** - 可能需要授权应用访问你的信息
4. **回调** - 自动跳转回应用,显示用户信息
5. **查看详情** - 点击 "View Full Profile" 查看完整用户信息
6. **登出** - 点击 "Logout" 退出登录

## 代码结构

```
main.go
├── handleHome        - 首页,显示登录状态
├── handleLogin       - 发起登录,重定向到 Zitadel
├── handleCallback    - 处理回调,验证 token
├── handleProfile     - 显示完整用户信息
└── handleLogout      - 登出并清除 session
```

## 关键概念

### OAuth2 Flow

1. 用户点击登录 → 生成 state → 重定向到授权页面
2. 用户授权 → Zitadel 回调带 code → 用 code 换 token
3. 验证 ID Token → 提取用户信息 → 保存到 session

### Token 类型

- **Access Token**: 用于调用 API 接口
- **ID Token**: JWT 格式,包含用户身份信息
- **Refresh Token**: 用于刷新 access token

### 安全措施

- ✅ State parameter 防止 CSRF 攻击
- ✅ ID Token 签名验证
- ✅ HTTPS 传输 (生产环境必须)
- ✅ Secure cookie 存储 session

## 常见问题

### 1. 回调失败 "redirect_uri mismatch"

检查 Zitadel 应用配置中的 Redirect URIs 是否包含 `http://localhost:3000/auth/callback`

### 2. ID Token 验证失败

确保 ISSUER_URL 配置正确,且 Zitadel 服务可访问

### 3. Session 丢失

检查 cookie 配置,确保 `store` 的密钥不为空

## 生产环境部署

1. 使用 HTTPS
2. 修改 session secret key
3. 配置正确的 redirect URL
4. 启用 PKCE (更安全)
5. 添加 token 刷新机制
6. 实现日志记录

## 扩展功能

- [ ] 添加 Refresh Token 自动刷新
- [ ] 添加角色和权限检查
- [ ] 集成到数据库存储用户信息
- [ ] 添加中间件保护路由
- [ ] 实现 API 接口认证

## 参考资料

- [Zitadel 官方文档](https://zitadel.com/docs)
- [OAuth2 规范](https://oauth.net/2/)
- [OIDC 规范](https://openid.net/connect/)
