package receivers

type DecryptFunc func(key string, fallback string) string
