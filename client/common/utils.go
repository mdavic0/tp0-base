package common

import "fmt"

const DELIMITER = ";"

type Apuesta struct {
	Nombre     string
	Apellido   string
	Documento  int
	Nacimiento string
	Numero     int
	IDAgencia  int
}

func (apuesta *Apuesta) ParseToString() string {
	return fmt.Sprintf("{Nombre:%s,Apellido:%s,Documento:%d,Nacimiento:%s,Numero:%d,IDAgencia:%d}",
		apuesta.Nombre,
		apuesta.Apellido,
		apuesta.Documento,
		apuesta.Nacimiento,
		apuesta.Numero,
		apuesta.IDAgencia,
	)
}
