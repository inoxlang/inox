package core

type DevAPI interface {
	GoValue
	PotentiallySharable
	DevAPI__()
}
