package gencobra

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/jcodybaker/go-shelly"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stoewer/go-strcase"
)

func RequestToCmd(req shelly.RPCRequestBody) (*cobra.Command, error) {
	c := &cobra.Command{}
	c.RunE = func(cmd *cobra.Command, args []string) error {
		if _, err := forEachStructField(reflect.ValueOf(req), "", newFlagReader(c.Flags(), req.Method())); err != nil {
			return err
		}
		if err := json.NewEncoder(os.Stdout).Encode(req); err != nil {
			return err
		}
		return nil

	}
	var err error
	if c.Use, err = transformMethodName(req.Method()); err != nil {
		return nil, err
	}
	if _, err := forEachStructField(reflect.ValueOf(req), "", newFlagFactory(c.Flags(), req.Method())); err != nil {
		return c, err
	}

	return c, nil
}

func transformMethodName(n string) (string, error) {
	_, subCommand, ok := strings.Cut(n, ".")
	if !ok {
		return "", errors.New("failed to parse method name")
	}
	return strcase.KebabCase(subCommand), nil
}

type something func(fieldType reflect.Type, fieldValue reflect.Value, name, prefix string) (bool, error)

func forEachStructField(v reflect.Value, prefix string, f something) (bool, error) {
	var mutatedStruct bool

	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if k := v.Kind(); k != reflect.Struct {
		return false, fmt.Errorf("expected struct, got: %T", v.Type)
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		fieldDef := t.Field(i)
		if !fieldDef.IsExported() {
			continue
		}
		jTag, ok := fieldDef.Tag.Lookup("json")
		if !ok {
			continue
		}
		name, _, _ := strings.Cut(jTag, ",")
		fieldType := fieldDef.Type
		fv := v.Field(i)

		if fieldType.Kind() == reflect.Struct ||
			(fieldType.Kind() == reflect.Pointer && (fieldType.Elem().Kind() == reflect.Struct)) {
			var newPrefix string
			if prefix != "" || name != "config" {
				if prefix != "" {
					newPrefix = prefix + "-"
				}
				newPrefix = newPrefix + strcase.KebabCase(name)
			}
			didSet := false
			if fieldType.Kind() == reflect.Pointer {
				fieldType = fieldType.Elem()
				if fv.IsNil() {
					fv = reflect.New(fieldType)
					didSet = true
				} else {
					fv = fv.Elem()
				}
			}
			shouldSetValue, err := forEachStructField(fv, newPrefix, f)
			if err != nil {
				return false, err
			}
			if shouldSetValue && didSet && fieldDef.Type.Kind() == reflect.Pointer {
				// the fv was mutated and we created it, we need to set it on the value.
				v.Field(i).Set(fv)
			}
		} else {
			fieldMutated, err := f(fieldType, fv, name, prefix)
			if err != nil {
				return false, err
			}
			if fieldMutated {
				mutatedStruct = true
			}
		}
	}
	return mutatedStruct, nil
}

func newFlagReader(f *pflag.FlagSet, method string) something {
	ff := func(typ reflect.Type, fieldValue reflect.Value, name, prefix string) (bool, error) {
		flagName := strcase.KebabCase(name)
		if prefix != "" {
			flagName = fmt.Sprintf("%s-%s", prefix, flagName)
		}
		fv := f.Lookup(flagName)
		if fv.Changed {
			var v interface{}
			var err error
			switch fv.Value.Type() {
			case "string":
				v, err = f.GetString(flagName)
			case "bool":
				v, err = f.GetBool(flagName)
			case "int":
				v, err = f.GetInt(flagName)
			case "int8":
				v, err = f.GetInt8(flagName)
			case "int16":
				v, err = f.GetInt16(flagName)
			case "int32":
				v, err = f.GetInt32(flagName)
			case "int64":
				v, err = f.GetInt64(flagName)
			case "uint":
				v, err = f.GetUint(flagName)
			case "uint8":
				v, err = f.GetUint8(flagName)
			case "uint16":
				v, err = f.GetUint16(flagName)
			case "uint32":
				v, err = f.GetUint32(flagName)
			case "uint64":
				v, err = f.GetUint64(flagName)
			default:
				return false, fmt.Errorf("unknown type: %v", typ.Kind())
			}
			if err != nil {
				return false, err
			}
			if fieldValue.Kind() == reflect.Pointer {
				if fieldValue.IsNil() {
					newValue := reflect.ValueOf(shelly.BoolPtr(true))
					fieldValue.Set(newValue)
				} else {
					fieldValue.Set(reflect.ValueOf(v))
				}
			}
		}
		return fv.Changed, nil
	}
	return ff
}

func newFlagFactory(f *pflag.FlagSet, method string) something {
	var ff something
	ff = func(typ reflect.Type, fieldValue reflect.Value, name, prefix string) (bool, error) {
		if typ.Kind() == reflect.Pointer {
			typ = typ.Elem()
			if fieldValue.IsNil() {
				fieldValue = reflect.New(typ)
			} else {
				fieldValue = fieldValue.Elem()
			}
		}
		flagName := strcase.KebabCase(name)
		if prefix != "" {
			flagName = fmt.Sprintf("%s-%s", prefix, flagName)
		}
		desc := fmt.Sprintf("Set the %q field on the %q request", name, method)
		if gotFlag := f.Lookup(flagName); gotFlag != nil {
			return false, nil
		}
		switch typ.Kind() {
		case reflect.Pointer:
			return ff(typ.Elem(), fieldValue.Elem(), name, prefix)
		case reflect.String:
			f.String(flagName, "", desc)
		case reflect.Bool:
			desc := fmt.Sprintf(
				"Set the %q field on the %q request; set --%s=false to disable.",
				name,
				method,
				flagName)
			f.Bool(flagName, false, desc)
		case reflect.Int:
			f.Int(flagName, 0, desc)
		case reflect.Int8:
			f.Int8(flagName, 0, desc)
		case reflect.Int16:
			f.Int16(flagName, 0, desc)
		case reflect.Int32:
			f.Int32(flagName, 0, desc)
		case reflect.Int64:
			f.Int64(flagName, 0, desc)
		case reflect.Uint:
			f.Uint(flagName, 0, desc)
		case reflect.Uint8:
			f.Uint8(flagName, 0, desc)
		case reflect.Uint16:
			f.Uint16(flagName, 0, desc)
		case reflect.Uint32:
			f.Uint32(flagName, 0, desc)
		case reflect.Uint64:
			f.Uint64(flagName, 0, desc)
		case reflect.Float32:
			f.Float32(flagName, 0, desc)
		case reflect.Float64:
			f.Float64(flagName, 0, desc)
		default:
			return false, fmt.Errorf("unknown type: %v", typ.Kind())
		}
		return false, nil
	}
	return ff
}
