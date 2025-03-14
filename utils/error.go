package utils

type Error string

func (e Error) Error() string {
	return string(e)
}

func NotNil(v any, e Error) {
	if v == nil {
		panic(e)
	}
}

func PanicOnError(err error) {
	if err != nil {
		panic(err)
	}
}
