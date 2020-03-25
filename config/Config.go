package config

type Config struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Time int `json:"time"`
	Blockclasslist []string `json:"blockclasslist"`
	Start string `json:"start"`
	End string `json:"end"`
	Logfile string `json:"logfile"`
}