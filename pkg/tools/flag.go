package tools

import (
	"flag"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/stoewer/go-strcase"
)

func lookupEnvOrValue[T any](envKey string, defaultVal T, parse func(val string) (T, error)) T {
	if val, ok := os.LookupEnv(envKey); ok {
		parsedVal, err := parse(val)
		if err != nil {
			log.Fatalf("Failed to parse %q environment variable value=%q as (%T): %v", envKey, val, defaultVal, err)
		}
		return parsedVal
	}
	return defaultVal
}

func lookupEnvOrString(envKey string, defaultVal string) string {
	return lookupEnvOrValue(envKey, defaultVal, func(val string) (string, error) {
		return val, nil
	})
}

func lookupEnvOrInt(envKey string, defaultVal int) int {
	return lookupEnvOrValue(envKey, defaultVal, strconv.Atoi)
}

func lookupEnvOrInt64(envKey string, defaultVal int64) int64 {
	return lookupEnvOrValue(envKey, defaultVal, func(val string) (int64, error) {
		return strconv.ParseInt(val, 10, 64)
	})
}

func lookupEnvOrDuration(envKey string, defaultVal time.Duration) time.Duration {
	return lookupEnvOrValue(envKey, defaultVal, time.ParseDuration)
}

func lookupEnvOrBool(envKey string, defaultVal bool) bool {
	return lookupEnvOrValue(envKey, defaultVal, strconv.ParseBool)
}

// StringVarFlagWithEnv allows to define string flag which default value
// is overridable when corresponding upper snake case environment variable is set.
func StringVarFlagWithEnv(p *string, name, value, usage string) {
	flag.StringVar(p, name, lookupEnvOrString(strcase.UpperSnakeCase(name), value), usage)
}

// IntVarFlagWithEnv allows to define int flag which default value
// is overridable when corresponding upper snake case environment variable is set.
func IntVarFlagWithEnv(p *int, name string, value int, usage string) {
	flag.IntVar(p, name, lookupEnvOrInt(strcase.UpperSnakeCase(name), value), usage)
}

// Int64VarFlagWithEnv allows to define int64 flag which default value
// is overridable when corresponding upper snake case environment variable is set.
func Int64VarFlagWithEnv(p *int64, name string, value int64, usage string) {
	flag.Int64Var(p, name, lookupEnvOrInt64(strcase.UpperSnakeCase(name), value), usage)
}

// DurationVarFlagWithEnv allows to define duration flag which default value
// is overridable when corresponding upper snake case environment variable is set.
func DurationVarFlagWithEnv(p *time.Duration, name string, value time.Duration, usage string) {
	flag.DurationVar(p, name, lookupEnvOrDuration(strcase.UpperSnakeCase(name), value), usage)
}

// BoolVarFlagWithEnv allows to define bool flag which default value
// is overridable when corresponding upper snake case environment variable is set.
func BoolVarFlagWithEnv(p *bool, name string, value bool, usage string) {
	flag.BoolVar(p, name, lookupEnvOrBool(strcase.UpperSnakeCase(name), value), usage)
}
