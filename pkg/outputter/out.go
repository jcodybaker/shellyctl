package outputter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"unicode"

	"github.com/rs/zerolog/log"
	"sigs.k8s.io/yaml"
)

type Outputter func(ctx context.Context, msg, field string, f any) error

// JSON encodes the data as JSON output to stdout.
func JSON(ctx context.Context, msg, field string, f any) error {
	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	return e.Encode(f)
}

// MinJSON encodes the data as minimized JSON output to stdout.
func MinJSON(ctx context.Context, msg, field string, f any) error {
	return json.NewEncoder(os.Stdout).Encode(f)
}

// Log encodes the data as a structured log.
func Log(ctx context.Context, msg, field string, f any) error {
	log.Ctx(ctx).Info().Any(field, f).Msg(msg)
	return nil
}

// Text encodes the data as a structured log.
func Text(ctx context.Context, msg, field string, f any) error {
	fmt.Printf("%s:\n", msg)
	text(reflect.ValueOf(f), "  ")
	return nil
}

func text(v reflect.Value, indent string) error {
	if v.Kind() == reflect.Pointer {
		text(v.Elem(), indent)
	}
	if v.Kind() == reflect.Struct {
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			fieldInfo := t.Field(i)
			if !fieldInfo.IsExported() {
				continue
			}
			fT := fieldInfo.Type
			fieldName := spaceDelimited(fieldInfo.Name)
			f := v.Field(i)
			if fT.Kind() == reflect.Pointer {
				if f.IsNil() {
					continue
				}
				f = f.Elem()
				fT = fT.Elem()
			}
			if fT.Kind() == reflect.Struct {
				fmt.Printf("%s%s:\n", indent, fieldName)
				if err := text(f, "  "+indent); err != nil {
					return err
				}
				continue
			}
			if fT.Kind() == reflect.Slice {
				b, err := json.Marshal(f.Interface())
				if err != nil {
					return fmt.Errorf("printing array: %w", err)
				}
				fmt.Printf("%s%s: %s\n", indent, fieldName, string(b))
			} else {
				fmt.Printf("%s%s: %v\n", indent, fieldName, f.Interface())
			}
		}
	}
	return nil
}

// YAML encodes the data as yaml output to stdout.
func YAML(ctx context.Context, msg, field string, f any) error {
	jsonBytes, err := json.Marshal(f)
	if err != nil {
		return err
	}
	yamlBytes, err := yaml.JSONToYAML(jsonBytes)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(yamlBytes)
	return err
}

// ByName selects an outputter by name or returns an error.
func ByName(name string) (Outputter, error) {
	switch name {
	case "json":
		return JSON, nil
	case "min-json":
		return MinJSON, nil
	case "yaml":
		return YAML, nil
	case "text":
		return Text, nil
	case "log":
		return Log, nil
	default:
		return nil, fmt.Errorf("unknown output formatter: %q", name)
	}
}

func spaceDelimited(s string) string {
	var out []string
	var last rune
	var wordStart int
	for i, r := range s {
		if unicode.IsUpper(r) && (last == rune(0) || !unicode.IsUpper(last)) {
			word := strings.TrimSpace(s[wordStart:i])
			if word != "" {
				out = append(out, word)
			}
			wordStart = i
		}
		last = r
	}
	word := strings.TrimSpace(s[wordStart:])
	if word != "" {
		out = append(out, word)
	}
	return strings.Join(out, " ")
}
