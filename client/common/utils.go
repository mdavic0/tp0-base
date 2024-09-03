package common

import "fmt"

const DELIMITER = ";"

type Bet struct {
	Name      string
	LastName  string
	Document  int
	BirthDate string
	Number    int
	Agency    int
}

func (bet *Bet) ParseToString() string {
	return fmt.Sprintf("{Name:%s,LastName:%s,Document:%d,BirthDate:%s,Number:%d,Agency:%d}",
		bet.Name,
		bet.LastName,
		bet.Document,
		bet.BirthDate,
		bet.Number,
		bet.Agency,
	)
}
