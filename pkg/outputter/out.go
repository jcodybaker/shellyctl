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
	fmt.Printf("%s:", msg)
	v := reflect.ValueOf(f)
	if (v.Kind() == reflect.Struct && v.NumField() == 0) ||
		(v.Kind() == reflect.Pointer && v.Elem().Kind() == reflect.Struct && v.Elem().NumField() == 0) {
		fmt.Println(" success")
		fmt.Println("")
		return nil
	}
	fmt.Println("")
	text(reflect.ValueOf(f), "  ", "  ")
	fmt.Println("")
	return nil
}

func text(v reflect.Value, firstIndent, indent string) error {
	if v.Kind() == reflect.Pointer {
		text(v.Elem(), firstIndent, indent)
	}
	if v.Kind() == reflect.Struct {
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			lineIndent := indent
			if i == 0 {
				lineIndent = firstIndent
			}
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
				fmt.Printf("%s%s:\n", lineIndent, fieldName)
				if err := text(f, "  "+indent, "  "+indent); err != nil {
					return err
				}
				continue
			}
			switch fT.Kind() {
			case reflect.Slice:
				if f.Len() == 0 {
					fmt.Printf("%s%s: NULL\n", lineIndent, fieldName)
				} else if fT.Elem().Kind() == reflect.Struct {
					fmt.Printf("%s%s:\n", lineIndent, fieldName)
					for j := 0; j < f.Len(); j++ {
						if err := text(f.Index(j), indent+"- ", indent+"  "); err != nil {
							return err
						}
					}
				} else if fT.Elem().Kind() == reflect.Pointer && fT.Elem().Elem().Kind() == reflect.Struct {
					fmt.Printf("%s%s:\n", lineIndent, fieldName)
					for j := 0; j < f.Len(); j++ {
						sliceValue := f.Index(j)
						if sliceValue.IsNil() {
							fmt.Printf("%s: NULL", indent+"  ")
						} else {
							if err := text(f.Index(j).Elem(), indent+"- ", indent+"  "); err != nil {
								return err
							}
						}
					}
				} else {
					b, err := json.Marshal(f.Interface())
					if err != nil {
						return fmt.Errorf("printing array: %w", err)
					}
					fmt.Printf("%s%s: %s\n", lineIndent, fieldName, string(b))
				}
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
				reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				fmt.Printf("%s%s: %d\n", lineIndent, fieldName, f.Interface())
			case reflect.Float32, reflect.Float64:
				fmt.Printf("%s%s: %f\n", lineIndent, fieldName, f.Interface())
			default:
				fmt.Printf("%s%s: %v\n", lineIndent, fieldName, f.Interface())
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
		isUpper := unicode.IsUpper(r)
		var nextIsLower, lastIsUpper bool
		if asRunes := []rune(s[i:]); len(asRunes) >= 2 && unicode.IsLower(asRunes[1]) {
			nextIsLower = true
		}
		if last != rune(0) && unicode.IsUpper(last) {
			lastIsUpper = true
		}

		if isUpper && (nextIsLower || !lastIsUpper) {
			word := strings.Trim(s[wordStart:i], " _-")
			if word != "" {
				out = append(out, word)
			}
			wordStart = i
		}
		last = r
	}
	word := strings.Trim(s[wordStart:], " _-")
	if word != "" {
		out = append(out, word)
	}
	return strings.Join(out, " ")
}
