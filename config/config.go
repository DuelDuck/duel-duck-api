package config

import (
	"time"

	"github.com/caarlos0/env/v11"
)

func NewConfig() *Config {
	config := env.Must[Config](env.ParseAs[Config]())

	return &config
}

const (
	EnvironmentProduction  = "prod"
	EnvironmentStage       = "stage"
	EnvironmentDevelopment = "dev"
)

type Config struct {
	HTTP     HTTPConfig
	Auth     AuthConfig
	Telegram TelegramConfig
	PG       DBConfig
	Redis    RedisConfig
	App      AppConfig
	Brevo    BrevoConfig
	Vault    VaultConfig
	Client   ClientConfig
	Duels    Duels `envPrefix:"DUELS_"`
}

type VaultConfig struct {
	RoleID       string `env:"VAULT_ROLE_ID,required"`
	SecretID     string `env:"VAULT_SECRET_ID,required"`
	VaultAddress string `env:"VAULT_ADDRESS,required"`
}

type HTTPConfig struct {
	PublicDomain        string `env:"HTTP_PUBLIC_DOMAIN,required"`
	Address             string `env:"HTTP_ADDRESS,required"`
	Host                string `env:"HTTP_HOST,required"`
	Port                string `env:"HTTP_PORT,required"`
	SwaggerValidatorURL string `env:"SWAGGER_VALIDATOR_URL,required"`
	AllowOrigins        string `env:"ALLOW_ORIGINS,required"`
	AllowCredentials    bool   `env:"ALLOW_CREDENTIALS,required"`
}

type AuthConfig struct {
	SecretSignKey      string        `env:"SECRET_SIGN_KEY,required"`
	RefreshTokenTTL    time.Duration `env:"REFRESH_TOKEN_TTL,required"`
	AccessTokenTTL     time.Duration `env:"ACCESS_TOKEN_TTL,required"`
	TelegramBotCodeTTL time.Duration `env:"TELEGRAM_BOT_CODE_TTL,required"`
}

type RedisConfig struct {
	Host     string `env:"REDIS_HOST,required"`
	DB       int    `env:"REDIS_DB,required"`
	Port     string `env:"REDIS_PORT,required"`
	Password string `env:"REDIS_PASSWORD,required"`
}

type BrevoConfig struct {
	BrevoSecretKey string `env:"BREVO_SECRET_KEY,required"`
	BrevoEmail     string `env:"BREVO_EMAIL,required"`
	BrevoName      string `env:"BREVO_NAME,required"`
}

type TelegramConfig struct {
	StatsBotTgAPIKey   string        `env:"STATS_BOT_TG_API_KEY,required"`
	WelcomeBotTgAPIKey string        `env:"WELCOME_BOT_TG_API_KEY,required"`
	MiniAppBotTgAPIKey string        `env:"MINIAPP_BOT_TG_API_KEY,required"`
	TournamentBot      TournamentBot `envPrefix:"TOURNAMENT_BOT_"`
	FAQBot             FAQBot        `envPrefix:"FAQ_BOT_"`
}

type ClientConfig struct {
	JupiterBaseURL string `env:"JUPITER_BASE_URL,required"`
	SolscanAPIKey  string `env:"SOLSCAN_API_KEY,required"`
	MoralisAPIKey  string `env:"MORALIS_API_KEY,required"`
}

type Duels struct {
	ShortTerm AutoCreatedDuels `envPrefix:"SHORT_TERM_"`
	LongTerm  AutoCreatedDuels `envPrefix:"LONG_TERM_"`

	ResolveJoinNotBefore time.Duration `env:"RESOLVE_JOIN_NOT_BEFORE,required"`
}
type AutoCreatedDuels struct {
	RunParams            string  `env:"RUN_PARAMS,required"`
	DuckPointPrice       uint64  `env:"DUCK_POINT_PRICE,required"`
	CryptoPrice          uint64  `env:"CRYPTO_PRICE,required"`
	DaysToEventDate      int     `env:"DAYS_TO_EVENT_DATE,required"`
	PriceDiffCoefficient float64 `env:"PRICE_DIFF_COEFFICIENT,required"`
	Commission           uint8   `env:"COMMISSION,required"`
}

// TODO: separate to several smaller configs

type AppConfig struct {
	Environment string `env:"ENVIRONMENT,required"`
	CMCApiKey   string `env:"CMC_API_KEY,required"`

	SolanaURL                    string `env:"SOLANA_URL,required"`
	SolanaAdminPrivateKey        string `env:"SOLANA_ADMIN_PRIVATE_KEY,required"`
	SolanaQuickNodeAPI           string `env:"SOLANA_QUICK_NODE_API"`
	SolanaPriorityUpdateInterval string `env:"SOLANA_PRIORITY_UPDATE_INTERVAL,required"`
	ContractAddress              string `env:"CONTRACT_ADDRESS,required"`
	ContractAddressAPI           string `env:"CONTRACT_ADDRESS_API,required"`

	FirebaseFilePath string `env:"FIREBASE_FILE_PATH,required"`

	WalletCacheEncryptionKey string        `env:"WALLET_CACHE_ENCRYPTION_KEY,required"`
	WalletCacheTTL           time.Duration `env:"WALLET_CACHE_TTL,required"`

	CryptoDuelsResolveInterval       string  `env:"CRYPTO_DUELS_RESOLVE_INTERVAL,required"`
	CryptoDuelsResolveBeforeInterval string  `env:"CRYPTO_DUELS_RESOLVE_BEFORE_INTERVAL,required"`
	SwapCommissionCoefficient        float64 `env:"SWAP_COMMISSION_COEFFICIENT,required"`

	ReferrerInvitationRewardDP uint64 `env:"REFERRER_INVITATION_REWARD_DP,required"`
}

type TournamentBot struct {
	BotTgAPIKey      string `env:"TG_API_KEY,required"`
	AllowTelegramIDs string `env:"ALLOW_TELEGRAM_IDS,required"`
}

type FAQBot struct {
	BotTgAPIKey string `env:"TG_API_KEY,required"`
}
