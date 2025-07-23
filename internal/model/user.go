package model

import (
	"strconv"
	"time"

	"github.com/google/uuid"
	initdata "github.com/telegram-mini-apps/init-data-golang"
	"github.com/uptrace/bun"

	"gitlab.com/duel-duck/duel-duck-api/pkg/mtype"
)

const (
	// Reward themes
	ThemeJustHatched            string = "Just Hatched"
	ThemeFeatherweightPredictor string = "Featherweight Predictor"
	ThemeSharpBill              string = "Sharp Bill"
	ThemeDuckBoss               string = "Duck Boss"
	ThemeOracleDuck             string = "Oracle Duck"
)

type User struct {
	bun.BaseModel `bun:"table:users,alias:u" json:"-"`

	ID                  uuid.UUID      `bun:",pk,type:uuid,default:uuid_generate_v4()" json:"id"`
	TelegramID          string         `bun:"type:varchar(255),unique,nullzero," json:"telegram_id"`
	Username            mtype.Username `bun:",type:varchar(17),notnull" json:"username"`
	Email               mtype.Email    `bun:",type:varchar(255),unique,nullzero" json:"email"`
	PublicAddress       string         `bun:",type:varchar(44),unique,nullzero" json:"public_address"`
	ImageUrl            string         `bun:",type:varchar(100)" json:"image_url"`
	BGUrl               string         `bun:",type:varchar(100)" json:"bg_url"`
	XURL                string         `bun:"x_url" json:"x_url"`
	InstagramURL        string         `bun:"instagram_url" json:"instagram_url"`
	YouTubeURL          string         `bun:"youtube_url" json:"youtube_url"`
	TelegramURL         string         `bun:"telegram_url" json:"telegram_url"`
	DiscordURL          string         `bun:"discord_url" json:"discord_url"`
	Website             string         `bun:"website" json:"website"`
	Bio                 string         `bun:",type:text" json:"bio"`
	TwoFa               bool           `bun:",default:false" json:"two_fa"`
	TwoFaSecret         string         `bun:",type:varchar(64)" json:"two_fa_secret"`
	Role                mtype.Role     `bun:",default:0,type:int" json:"role"`
	IsPremium           bool           `bun:",notnull,default:false" json:"is_premium"`
	Level               uint32         `bun:",notnull,type:int,default:0" json:"level"`
	CurrentXP           uint32         `bun:",notnull,type:int,default:0" json:"current_xp"`
	Balance             mtype.Balance  `bun:",notnull,default:0" json:"balance"`
	USDCAutoswap        bool           `bun:",notnull,default:false" json:"usdc_autoswap"`
	SolAutoswap         bool           `bun:",notnull,default:false" json:"sol_autoswap"`
	ReferralToken       string         `bun:",type:varchar(32)" json:"referral_token"`
	DailyRewardStreak   int            `bun:",notnull,default:1,type:int" json:"daily_reward_streak"`
	LastCompletedStreak time.Time      `bun:",notnull" json:"last_completed_streak"`
	CreatedAt           time.Time      `bun:",notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt           time.Time      `bun:",notnull,default:current_timestamp" json:"updated_at"`
}

type UserStats struct {
	bun.BaseModel `bun:"table:users,alias:u"`

	ID          uuid.UUID `bun:",type:uuid" json:"id"`
	RewardTheme string    `bun:"-" json:"reward_theme"`
	Level       uint32    `bun:",notnull,type:int,default:0" json:"level"`
	CurrentXP   uint32    `bun:",notnull,type:int,default:0" json:"current_xp"`
	NextLevelXP uint32    `json:"next_level_xp"`
	DDPEarned   uint64    `bun:"ddp_earned" json:"ddp_earned"`
	USDCEarned  uint64    `bun:"usdc_earned" json:"usdc_earned"`
}

type UserWallet struct {
	bun.BaseModel `bun:"table:users,alias:u" json:"-"`

	ID            uuid.UUID `bun:",pk,type:uuid,default:uuid_generate_v4()" json:"id"`
	PublicAddress string    `bun:",type:varchar(44),unique,nullzero" json:"public_address"`
}

type UserEditReq struct {
	bun.BaseModel `bun:"table:users,alias:u" json:"-"`

	USDCAutoswap *bool   `bun:"usdc_autoswap" json:"usdc_autoswap"`
	SolAutoswap  *bool   `bun:"sol_autoswap" json:"sol_autoswap"`
	XURL         *string `bun:"x_url" json:"x_url"`
	InstagramURL *string `bun:"instagram_url" json:"instagram_url"`
	YouTubeURL   *string `bun:"youtube_url" json:"youtube_url"`
	TelegramURL  *string `bun:"telegram_url" json:"telegram_url"`
	DiscordURL   *string `bun:"discord_url" json:"discord_url"`
	Website      *string `bun:"website" json:"website"`
}

type UsernameChange struct {
	bun.BaseModel `bun:"table:users,alias:u" json:"-"`

	Username mtype.Username `bun:"username" json:"username"`
}

func NewUser(
	username mtype.Username,
	referralToken string,
	profileImageURL string,
) *User {
	now := time.Now().UTC()
	return &User{
		ID:            uuid.New(),
		Username:      username,
		ReferralToken: referralToken,
		ImageUrl:      profileImageURL,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

type AuthWithWallet struct {
	Address    string `json:"address" binding:"required"`
	WalletName string `json:"wallet_name" binding:"required"`

	// Secret is the signature of the public address
	Secret        string `json:"secret" binding:"required"`
	ReferrerToken string `json:"referrer_token"`
}

type SendCode struct {
	Email string `json:"email"`
}

type SignInWithEmail struct {
	Email               string `json:"email"`
	Code                string `json:"code"`
	ReferrerToken       string `json:"referrer_token"`
	AdvertiserLinkToken string `json:"advertiser_link_token"`
}

type SignInWithGoogle struct {
	ReferrerToken       string `json:"referrer_token"`
	AdvertiserLinkToken string `json:"advertiser_link_token"`
}

type SignInWithTelegramMiniAppReq struct {
	ReferrerToken       string `json:"referral_token"`
	AdvertiserLinkToken string `json:"advertiser_link_token"`
	InitDataRaw         string `json:"init_data_raw"`
}

type SignInWithTelegramMiniApp struct {
	TelegramID          string `json:"telegram_id"`
	ReferrerToken       string `json:"referral_token"`
	AdvertiserLinkToken string `json:"advertiser_link_token"`
	IsPremium           bool   `json:"is_premium"`
	FirstName           string `json:"first_name"`
	LastName            string `json:"last_name"`
	Username            string `json:"username"`
	InitDataRaw         string `json:"init_data_raw"`
}

func NewSignInWithTelegramMiniApp(
	data initdata.InitData,
	authTGReq SignInWithTelegramMiniAppReq,
) SignInWithTelegramMiniApp {
	return SignInWithTelegramMiniApp{
		TelegramID:    strconv.FormatInt(data.User.ID, 10),
		IsPremium:     data.User.IsPremium,
		FirstName:     data.User.FirstName,
		LastName:      data.User.LastName,
		Username:      data.User.Username,
		ReferrerToken: authTGReq.ReferrerToken,
	}
}

type SignInWithTelegramWeb struct {
	TelegramID string    `json:"telegram_id"`
	Code       uuid.UUID `json:"code"`
}

type SignInWithTelegramWebCache struct {
	Code        uuid.UUID `json:"code"`
	ReadCounter uint8     `json:"read_counter"`
}

func GetRewardTheme(lvl uint32) string {
	if lvl >= 1 && lvl <= 5 {
		return ThemeJustHatched
	} else if lvl >= 6 && lvl <= 15 {
		return ThemeFeatherweightPredictor
	} else if lvl >= 16 && lvl <= 30 {
		return ThemeSharpBill
	} else if lvl >= 31 && lvl <= 50 {
		return ThemeDuckBoss
	} else if lvl > 50 {
		return ThemeOracleDuck
	} else {
		return ""
	}
}
