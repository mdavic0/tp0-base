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

func (apuesta *Bet) ParseToString() string {
	return fmt.Sprintf("Name:%s,LastName:%s,Document:%d,BirthDate:%s,Number:%d,Agency:%d",
		apuesta.Name,
		apuesta.LastName,
		apuesta.Document,
		apuesta.BirthDate,
		apuesta.Number,
		apuesta.Agency,
	)
}
