package config

type Config struct {
	Precision  string `env:"PRECISION"`
	Tokens     string `env:"TOKENS"`
	Url        string `env:"URL"`
	Alchemy    string `env:"ALCHEMY"`
	Contract   string `env:"CONTRACT"`
	PrivateKey string `env:"PRIVATEKEY"`
}
