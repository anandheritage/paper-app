package usecase

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/paper-app/backend/internal/config"
	"github.com/paper-app/backend/internal/domain"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmailExists        = errors.New("email already exists")
	ErrInvalidToken       = errors.New("invalid token")
	ErrTokenExpired       = errors.New("token expired")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidGoogleToken = errors.New("invalid google token")
)

type AuthUsecase struct {
	userRepo  domain.UserRepository
	tokenRepo domain.RefreshTokenRepository
	cfg       *config.JWTConfig
	googleCfg *config.GoogleConfig
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
}

type Claims struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	jwt.RegisteredClaims
}

func NewAuthUsecase(userRepo domain.UserRepository, tokenRepo domain.RefreshTokenRepository, cfg *config.JWTConfig, googleCfg *config.GoogleConfig) *AuthUsecase {
	return &AuthUsecase{
		userRepo:  userRepo,
		tokenRepo: tokenRepo,
		cfg:       cfg,
		googleCfg: googleCfg,
	}
}

func (u *AuthUsecase) Register(email, password, name string) (*domain.User, *TokenPair, error) {
	existing, err := u.userRepo.GetByEmail(email)
	if err != nil {
		return nil, nil, err
	}
	if existing != nil {
		return nil, nil, ErrEmailExists
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, nil, err
	}

	user := &domain.User{
		Email:        email,
		PasswordHash: string(hashedPassword),
		Name:         name,
		AuthProvider: "email",
	}

	if err := u.userRepo.Create(user); err != nil {
		return nil, nil, err
	}

	tokens, err := u.generateTokenPair(user)
	if err != nil {
		return nil, nil, err
	}

	return user, tokens, nil
}

func (u *AuthUsecase) Login(email, password string) (*domain.User, *TokenPair, error) {
	user, err := u.userRepo.GetByEmail(email)
	if err != nil {
		return nil, nil, err
	}
	if user == nil {
		return nil, nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	tokens, err := u.generateTokenPair(user)
	if err != nil {
		return nil, nil, err
	}

	return user, tokens, nil
}

// GoogleUserInfo represents the response from Google's userinfo endpoint
type GoogleUserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
}

func (u *AuthUsecase) GoogleLogin(accessToken string) (*domain.User, *TokenPair, error) {
	// Verify the Google access token by fetching user info
	userInfo, err := u.fetchGoogleUserInfo(accessToken)
	if err != nil {
		return nil, nil, ErrInvalidGoogleToken
	}

	tokenInfo := userInfo

	// Check if user already exists with this Google ID
	user, err := u.userRepo.GetByProviderID("google", tokenInfo.Sub)
	if err != nil {
		return nil, nil, err
	}

	if user == nil {
		// Check if email is already registered
		user, err = u.userRepo.GetByEmail(tokenInfo.Email)
		if err != nil {
			return nil, nil, err
		}

		if user != nil {
			// Link Google to existing account
			user.AuthProvider = "google"
			user.ProviderID = tokenInfo.Sub
			if user.Name == "" {
				user.Name = tokenInfo.Name
			}
			if err := u.userRepo.Update(user); err != nil {
				return nil, nil, err
			}
		} else {
			// Create new user
			user = &domain.User{
				Email:        tokenInfo.Email,
				Name:         tokenInfo.Name,
				AuthProvider: "google",
				ProviderID:   tokenInfo.Sub,
			}
			if err := u.userRepo.Create(user); err != nil {
				return nil, nil, err
			}
		}
	}

	tokens, err := u.generateTokenPair(user)
	if err != nil {
		return nil, nil, err
	}

	return user, tokens, nil
}

func (u *AuthUsecase) fetchGoogleUserInfo(accessToken string) (*GoogleUserInfo, error) {
	req, err := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v3/userinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, ErrInvalidGoogleToken
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var userInfo GoogleUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, err
	}

	if userInfo.Email == "" || userInfo.Sub == "" {
		return nil, ErrInvalidGoogleToken
	}

	return &userInfo, nil
}

func (u *AuthUsecase) RefreshToken(refreshToken string) (*TokenPair, error) {
	tokenHash := hashToken(refreshToken)

	storedToken, err := u.tokenRepo.GetByTokenHash(tokenHash)
	if err != nil {
		return nil, err
	}
	if storedToken == nil {
		return nil, ErrInvalidToken
	}

	if storedToken.ExpiresAt.Before(time.Now()) {
		u.tokenRepo.DeleteByTokenHash(tokenHash)
		return nil, ErrTokenExpired
	}

	user, err := u.userRepo.GetByID(storedToken.UserID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	// Delete old refresh token
	u.tokenRepo.DeleteByTokenHash(tokenHash)

	return u.generateTokenPair(user)
}

func (u *AuthUsecase) Logout(refreshToken string) error {
	tokenHash := hashToken(refreshToken)
	return u.tokenRepo.DeleteByTokenHash(tokenHash)
}

func (u *AuthUsecase) ValidateAccessToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(u.cfg.Secret), nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}

func (u *AuthUsecase) GetUserByID(id uuid.UUID) (*domain.User, error) {
	return u.userRepo.GetByID(id)
}

func (u *AuthUsecase) generateTokenPair(user *domain.User) (*TokenPair, error) {
	// Generate access token
	expiresAt := time.Now().Add(u.cfg.AccessExpiry)
	claims := &Claims{
		UserID: user.ID,
		Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   user.ID.String(),
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessTokenString, err := accessToken.SignedString([]byte(u.cfg.Secret))
	if err != nil {
		return nil, err
	}

	// Generate refresh token
	refreshToken := uuid.New().String()
	refreshTokenHash := hashToken(refreshToken)

	storedRefreshToken := &domain.RefreshToken{
		UserID:    user.ID,
		TokenHash: refreshTokenHash,
		ExpiresAt: time.Now().Add(u.cfg.RefreshExpiry),
	}

	if err := u.tokenRepo.Create(storedRefreshToken); err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt.Unix(),
	}, nil
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
