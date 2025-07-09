package config

import "fmt"

type DBConfig struct {
	User     string `env:"POSTGRES_USER,required"`
	Password string `env:"POSTGRES_PASSWORD,required"`
	Port     string `env:"POSTGRES_PORT,required"`
	Host     string `env:"POSTGRES_HOST,required"`
	Database string `env:"POSTGRES_DATABASE,required"`
	SSLMode  string `env:"POSTGRES_SSLMODE,required"`
}

func (c *DBConfig) GetConnectString() string {
	info := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		c.Host,
		c.Port,
		c.User,
		c.Password,
		c.Database,
	)

	return info
}
