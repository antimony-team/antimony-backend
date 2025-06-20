package auth

import (
	"antimonyBackend/config"
	"antimonyBackend/utils"
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/charmbracelet/log"
	"github.com/coreos/go-oidc"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
)

const NativeUserID = "00000000-0000-0000-0000-00000000000"

type (
	AuthManager interface {
		CreateAuthToken(userId string) (string, error)
		CreateAccessToken(authUser AuthenticatedUser) (string, error)
		AuthenticateUser(tokenString string) (*AuthenticatedUser, error)
		LoginNative(username string, password string) (string, string, error)
		GetAuthCodeURL(stateToken string) (string, error)
		AuthenticateWithCode(
			authCode string,
			userSubToIdMapper func(userSub string, userProfile string) (string, error),
		) (*AuthenticatedUser, error)
		AuthenticatorMiddleware() gin.HandlerFunc
		RefreshAccessToken(authToken string) (string, error)
		RegisterTestUser(authUser AuthenticatedUser) (string, error)

		GetAuthConfig() AuthConfig
	}

	authManager struct {
		config             *config.AntimonyConfig
		authenticatedUsers map[string]*AuthenticatedUser
		oauth2Config       oauth2.Config
		provider           oidc.Provider
		oidcSecret         string
		jwtSecret          []byte
		adminGroups        []string
		authConfig         AuthConfig
		nativeUsername     string
		nativePassword     string
	}

	AuthenticatedUser struct {
		// The UUID of the user
		UserId string
		// List of the names of collections that the user has access to
		Collections []string
		IsAdmin     bool
	}

	AuthConfig struct {
		OpenId OpenIdAuthConfig `json:"openId"`
		Native NativeAuthConfig `json:"native"`
	}

	OpenIdAuthConfig struct {
		Enabled bool `json:"enabled"`
	}

	NativeAuthConfig struct {
		Enabled    bool `json:"enabled"`
		AllowEmpty bool `json:"allowEmpty"`
	}
)

func CreateAuthManager(config *config.AntimonyConfig) AuthManager {
	isOpenIdEnabled := config.Auth.EnableOpenID
	isNativeEnabled := config.Auth.EnableNative

	if !isNativeEnabled && !isOpenIdEnabled {
		log.Fatal(
			"Please enable at least one authentication method.",
			"native",
			isNativeEnabled,
			"openid",
			isOpenIdEnabled,
		)
	}

	nativeUsername := os.Getenv("SB_NATIVE_USERNAME")
	nativePassword := os.Getenv("SB_NATIVE_PASSWORD")
	emptyCredentials := false

	if isNativeEnabled && (nativeUsername == "" || nativePassword == "") {
		emptyCredentials = true
		log.Warn("Native authentication is enabled but username or password are empty!")
	} else {
		log.Info("Native authentication is enabled.", "username", nativeUsername)
	}

	authConfig := AuthConfig{
		OpenId: OpenIdAuthConfig{
			Enabled: isOpenIdEnabled,
		},
		Native: NativeAuthConfig{
			Enabled:    isNativeEnabled,
			AllowEmpty: emptyCredentials,
		},
	}

	authManager := &authManager{
		config:             config,
		authenticatedUsers: make(map[string]*AuthenticatedUser),
		adminGroups:        config.Auth.OpenIdAdminGroups,
		jwtSecret:          ([]byte)(rand.Text()),
		oidcSecret:         os.Getenv("SB_OIDC_SECRET"),
		authConfig:         authConfig,
		nativeUsername:     nativeUsername,
		nativePassword:     nativePassword,
	}

	if isNativeEnabled {
		authManager.authenticatedUsers[NativeUserID] = &AuthenticatedUser{
			UserId:      NativeUserID,
			IsAdmin:     true,
			Collections: make([]string, 0),
		}
	}

	if isOpenIdEnabled {
		provider, err := oidc.NewProvider(context.TODO(), config.Auth.OpenIdIssuer)
		if err != nil {
			log.Fatalf("Failed to connect to OpenID provider: %s", err.Error())
		} else {
			authManager.provider = *provider
			authManager.oauth2Config = oauth2.Config{
				ClientID:     config.Auth.OpenIdClientID,
				ClientSecret: authManager.oidcSecret,
				RedirectURL:  fmt.Sprintf("%s/users/login/success", config.Auth.OpenIdRedirectHost),
				Endpoint:     provider.Endpoint(),
				Scopes:       []string{oidc.ScopeOpenID},
			}
		}
	}

	return authManager
}

func (m *authManager) RefreshAccessToken(authToken string) (string, error) {
	var (
		authUser       *AuthenticatedUser
		newAccessToken string
		err            error
	)

	if authUser, err = m.AuthenticateUser(authToken); err != nil {
		return "", err
	} else if newAccessToken, err = m.CreateAccessToken(*authUser); err != nil {
		return "", err
	} else {
		return newAccessToken, nil
	}
}

func (m *authManager) AuthenticatorMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var (
			accessToken string
			user        *AuthenticatedUser
			err         error
		)

		accessToken, err = ctx.Cookie("accessToken")
		if err != nil {
			ctx.JSON(utils.CreateErrorResponse(utils.ErrUnauthorized))
			ctx.Abort()
			return
		}

		if user, err = m.AuthenticateUser(accessToken); err != nil {
			ctx.JSON(utils.CreateErrorResponse(utils.ErrTokenInvalid))
			ctx.Abort()
			return
		} else {
			ctx.Set("authUser", *user)
			ctx.Next()
		}
	}
}

func (m *authManager) AuthenticateWithCode(
	authCode string,
	userSubToIdMapper func(userSub string, userProfile string) (string, error),
) (*AuthenticatedUser, error) {
	if !m.authConfig.OpenId.Enabled {
		return nil, utils.ErrOpenIDAuthDisabledError
	}

	ctx := context.TODO()
	token, err := m.oauth2Config.Exchange(ctx, authCode)
	if err != nil {
		log.Errorf("[AUTH] OAuth token exchange failed: %s", err.Error())
		return nil, utils.ErrOpenIDError
	}

	info, err := m.provider.UserInfo(ctx, m.oauth2Config.TokenSource(ctx, token))
	if err != nil {
		log.Errorf("[AUTH] Failed to get oauth userinfo: %s", err.Error())
		return nil, utils.ErrOpenIDError
	}

	var claims struct {
		Sub     string   `json:"sub"`
		Groups  []string `json:"groups"`
		Profile string   `json:"email"`
	}

	err = info.Claims(&claims)
	if err != nil {
		log.Warn("[AUTH] Failed to parse claims from userinfo: %s", err.Error())
		return nil, utils.ErrOpenIDError
	}

	userSub := claims.Sub
	userGroups := claims.Groups
	userProfile := claims.Profile

	isAdmin := false
	for _, group := range m.adminGroups {
		if slices.Contains(userGroups, group) {
			isAdmin = true
			break
		}
	}

	// Register authenticated user
	userId, err := userSubToIdMapper(userSub, userProfile)
	if err != nil {
		return nil, err
	}

	authenticatedUser := &AuthenticatedUser{
		UserId:      userId,
		IsAdmin:     isAdmin,
		Collections: userGroups,
	}
	m.authenticatedUsers[userId] = authenticatedUser

	return authenticatedUser, nil
}
func (m *authManager) GetAuthCodeURL(stateToken string) (string, error) {
	if !m.authConfig.OpenId.Enabled {
		return "", utils.ErrOpenIDAuthDisabledError
	}

	return m.oauth2Config.AuthCodeURL(stateToken), nil
}

func (m *authManager) LoginNative(username string, password string) (string, string, error) {
	var (
		authToken   string
		accessToken string
		err         error
	)

	if !m.authConfig.Native.Enabled {
		return "", "", utils.ErrNativeAuthDisabledError
	}

	if username == m.nativeUsername && password == m.nativePassword {
		authUser := m.authenticatedUsers[NativeUserID]
		if authToken, err = m.CreateAuthToken(NativeUserID); err != nil {
			return "", "", err
		} else if accessToken, err = m.CreateAccessToken(*authUser); err != nil {
			return "", "", err
		} else {
			return authToken, accessToken, nil
		}
	}
	return "", "", utils.ErrInvalidCredentials
}

func (m *authManager) AuthenticateUser(tokenString string) (*AuthenticatedUser, error) {
	if token, err := jwt.Parse(tokenString, m.tokenParser); err != nil {
		return nil, utils.ErrTokenInvalid
	} else if tokenClaims, ok := token.Claims.(jwt.MapClaims); !ok {
		return nil, utils.ErrTokenInvalid
	} else if userId, ok := tokenClaims["id"]; !ok {
		return nil, utils.ErrTokenInvalid
	} else {
		userIdStr, ok := userId.(string)
		if !ok {
			return nil, utils.ErrTokenInvalid
		}

		if permissions, ok := m.authenticatedUsers[userIdStr]; !ok {
			return nil, utils.ErrTokenInvalid
		} else {
			return permissions, nil
		}
	}
}

func (m *authManager) CreateAuthToken(userId string) (string, error) {
	sbToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":  userId,
		"nbf": time.Now().Unix(),
		"exp": time.Now().Add(time.Hour * 720).Unix(),
	})

	return sbToken.SignedString(m.jwtSecret)
}

func (m *authManager) CreateAccessToken(authUser AuthenticatedUser) (string, error) {
	sbToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":      authUser.UserId,
		"isAdmin": authUser.IsAdmin,
		"nbf":     time.Now().Unix(),
		"exp":     time.Now().Add(time.Minute * 30).Unix(),
	})

	return sbToken.SignedString(m.jwtSecret)
}

func (m *authManager) tokenParser(token *jwt.Token) (interface{}, error) {
	if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, utils.ErrTokenInvalid
	}

	return m.jwtSecret, nil
}

func (m *authManager) RegisterTestUser(user AuthenticatedUser) (string, error) {
	m.authenticatedUsers[user.UserId] = &user
	return user.UserId, nil
}

func (m *authManager) GetAuthConfig() AuthConfig {
	return m.authConfig
}
