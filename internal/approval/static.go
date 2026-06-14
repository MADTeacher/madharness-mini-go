package approval

// StaticPrompter нужен тестам и embedding-сценариям без настоящего stdin.
type StaticPrompter struct {
	Decision Decision
	Err      error
}

// Prompt возвращает заранее заданное решение.
func (p StaticPrompter) Prompt(Request) (Decision, error) {
	return p.Decision, p.Err
}
