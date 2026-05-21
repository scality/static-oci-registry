package config

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"reflect"
	"text/tabwriter"
	"time"

	"github.com/scality/go-errors"

	"github.com/sethvargo/go-envconfig"
)

// ApplicationVersion is the version of the application.
// It is set at build time using ldflags.
//
//nolint:gochecknoglobals // This is a constant.
var ApplicationVersion = "dev"

const ApplicationName = "static-oci-registry"

type (
	Environment struct {
		LogLevel string `env:"LOG_LEVEL, default=info"`
		HTTP     HTTP   `env:",prefix=HTTP_"`
		FS       FS     `env:",prefix=FS_"`
	}
	HTTP struct {
		Addr string `env:"ADDR, default=:5000"`
		TLS  TLS    `env:",prefix=TLS_"`
	}
	TLS struct {
		CertFilePath string `env:"CERT_FILE_PATH"`
		KeyFilePath  string `env:"KEY_FILE_PATH"`
	}
	FS struct {
		Root string `env:"ROOT, default=/data"`
	}
)

func NewEnvironment(ctx context.Context) (*Environment, error) {
	cfg := &Environment{}

	err := cfg.Load(ctx)
	if err != nil {
		return nil, errors.Wrap(err, errors.WithDetail("failed loading environment variables"))
	}

	return cfg, nil
}

func (cfg *Environment) Load(ctx context.Context) error {
	err := envconfig.Process(ctx, cfg)

	defer fmt.Println(cfg.ToString())

	if err != nil {
		return errors.Wrap(err, errors.WithDetail("failed loading config"))
	}

	return nil
}

//nolint:revive // let's ignore error checks here
func (cfg *Environment) ToString() string {
	b := &bytes.Buffer{}

	writer := tabwriter.NewWriter(b, 0, 0, 1, ' ', tabwriter.Debug)
	write(writer, cfg, 0)
	writer.Flush()

	return b.String()
}

//nolint:gocognit,revive // Function is a bit complex but that's fine
func write(writer io.Writer, src any, level int) {
	value := reflect.ValueOf(src)

	if value.Kind() != reflect.Struct {
		value = value.Elem()
	}

	prefix := ""
	for range level {
		prefix += "  "
	}

	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)

		if !field.CanInterface() {
			continue
		}

		typeField := value.Type().Field(i)

		if field.Kind() == reflect.Struct && field.Type() != reflect.TypeOf(time.Time{}) {
			fmt.Fprintf(writer, "%s%s:\t\t\n", prefix, typeField.Name)
			write(writer, field.Interface(), level+1)

			continue
		}

		val := field.Interface()
		if v, ok := typeField.Tag.Lookup("secret"); ok && v == "true" {
			val = "********"
		}

		fmt.Fprintf(writer, "%s%s\t %v\t %s\n", prefix, typeField.Name, val, field.Type().String())
	}
}
