package common

import "fmt"

type Bet struct {
	Nombre     string
	Apellido   string
	DNI        string
	Nacimiento string
	Numero     string
}

func (b Bet) Serialize(agencyID string) string {
	return fmt.Sprintf("{agency:%s,nombre:%s,apellido:%s,dni:%s,nacimiento:%s,numero:%s}",
		agencyID, b.Nombre, b.Apellido, b.DNI, b.Nacimiento, b.Numero)
}
