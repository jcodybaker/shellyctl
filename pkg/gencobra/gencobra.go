package gencobra

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"

	"github.com/jcodybaker/go-shelly"
	"github.com/jcodybaker/shellyctl/pkg/discovery"
	"github.com/jcodybaker/shellyctl/pkg/outputter"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stoewer/go-strcase"
)

type Baggage struct {
	Discoverer *discovery.Discoverer
	Output     outputter.Outputter
}

type Component struct {
	Parent   *cobra.Command
	Requests []shelly.RPCRequestBody
}

func ComponentsToCmd(components []*Component, baggage *Baggage) error {
	for _, c := range components {
		for _, r := range c.Requests {
			cmd, err := RequestToCmd(r, baggage)
			if err != nil {
				return err
			}
			c.Parent.AddCommand(cmd)
		}
	}
	return nil
}

func RequestToCmd(req shelly.RPCRequestBody, baggage *Baggage) (*cobra.Command, error) {
	c := &cobra.Command{}
	c.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := c.Context()
		ll := log.Ctx(ctx).With().Str("request", req.Method()).Logger()
		if _, err := forEachStructField(reflect.ValueOf(req), "", newFlagReader(c.Flags(), req.Method())); err != nil {
			return err
		}

		if _, err := baggage.Discoverer.Search(ctx); err != nil {
			return err
		}

		for _, d := range baggage.Discoverer.AllDevices() {
			ll := d.Log(ll)
			ll.Info().Any("request_body", req).Str("method", req.Method()).Msg("sending request")
			conn, err := d.Open(ctx)
			if err != nil {
				return err
			}
			defer func() {
				if err := conn.Disconnect(ctx); err != nil {
					ll.Warn().Err(err).Msg("disconnecting from device")
				}
			}()
			resp := req.NewResponse()
			reqContext := ctx
			cancel := func() {} // no-op
			if dur := viper.GetDuration("rpc-timeout"); dur != 0 {
				reqContext, cancel = context.WithTimeout(ctx, dur)
			}
			raw, err := shelly.Do(reqContext, conn, d.AuthCallback(ctx), req, resp)
			cancel()
			if err != nil {
				if viper.GetBool("skip-failed-hosts") {
					ll.Err(err).Msg("error executing request; contining because --skip-failed-hosts=true")
					continue
				} else {
					ll.Fatal().Err(err).Msg("error executing request")
				}
			}
			ll.Debug().RawJSON("raw_response", raw.Response).Msg("got raw response")
			baggage.Output(
				ctx,
				fmt.Sprintf("Response to %s command for %s", req.Method(), d.BestName()),
				"response",
				resp,
				raw.Response,
			)
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

type fieldFunc func(fieldType reflect.Type, fieldValue reflect.Value, name, prefix string) (bool, error)

func forEachStructField(v reflect.Value, prefix string, f fieldFunc) (bool, error) {
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
				mutatedStruct = true
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

func newFlagReader(f *pflag.FlagSet, method string) fieldFunc {
	ff := func(typ reflect.Type, fieldValue reflect.Value, name, prefix string) (bool, error) {
		flagName := strcase.KebabCase(name)
		if prefix != "" {
			flagName = fmt.Sprintf("%s-%s", prefix, flagName)
		}
		fv := f.Lookup(flagName)
		if fv.Changed {
			var err error
			switch fieldValue.Type() {
			case reflect.TypeOf(true):
				var b bool
				b, err = f.GetBool(flagName)
				fieldValue.SetBool(b)
			case reflect.TypeOf((*bool)(nil)):
				var b bool
				b, err = f.GetBool(flagName)
				fieldValue.Set(reflect.ValueOf(&b))
			case reflect.TypeOf(float32(0)):
				var i float32
				i, err = f.GetFloat32(flagName)
				fieldValue.SetFloat(float64(i))
			case reflect.TypeOf((*float32)(nil)):
				var i float32
				i, err = f.GetFloat32(flagName)
				fieldValue.Set(reflect.ValueOf(&i))
			case reflect.TypeOf(float64(0)):
				var i float64
				i, err = f.GetFloat64(flagName)
				fieldValue.SetFloat(float64(i))
			case reflect.TypeOf((*float64)(nil)):
				var i float64
				i, err = f.GetFloat64(flagName)
				fieldValue.Set(reflect.ValueOf(&i))
			case reflect.TypeOf(int(0)):
				var i int
				i, err = f.GetInt(flagName)
				fieldValue.SetInt(int64(i))
			case reflect.TypeOf((*int)(nil)):
				var i int
				i, err = f.GetInt(flagName)
				fieldValue.Set(reflect.ValueOf(&i))
			case reflect.TypeOf(int8(0)):
				var i int8
				i, err = f.GetInt8(flagName)
				fieldValue.SetInt(int64(i))
			case reflect.TypeOf((*int8)(nil)):
				var i int8
				i, err = f.GetInt8(flagName)
				fieldValue.Set(reflect.ValueOf(&i))
			case reflect.TypeOf(int16(0)):
				var i int16
				i, err = f.GetInt16(flagName)
				fieldValue.SetInt(int64(i))
			case reflect.TypeOf((*int16)(nil)):
				var i int16
				i, err = f.GetInt16(flagName)
				fieldValue.Set(reflect.ValueOf(&i))
			case reflect.TypeOf(int32(0)):
				var i int32
				i, err = f.GetInt32(flagName)
				fieldValue.SetInt(int64(i))
			case reflect.TypeOf((*int32)(nil)):
				var i int32
				i, err = f.GetInt32(flagName)
				fieldValue.Set(reflect.ValueOf(&i))
			case reflect.TypeOf(int64(0)):
				var i int64
				i, err = f.GetInt64(flagName)
				fieldValue.SetInt(i)
			case reflect.TypeOf((*int64)(nil)):
				var i int64
				i, err = f.GetInt64(flagName)
				fieldValue.Set(reflect.ValueOf(&i))
			case reflect.TypeOf(uint(0)):
				var i uint
				i, err = f.GetUint(flagName)
				fieldValue.SetUint(uint64(i))
			case reflect.TypeOf((*uint)(nil)):
				var i uint
				i, err = f.GetUint(flagName)
				fieldValue.Set(reflect.ValueOf(&i))
			case reflect.TypeOf(uint8(0)):
				var i uint8
				i, err = f.GetUint8(flagName)
				fieldValue.SetUint(uint64(i))
			case reflect.TypeOf((*uint8)(nil)):
				var i uint8
				i, err = f.GetUint8(flagName)
				fieldValue.Set(reflect.ValueOf(&i))
			case reflect.TypeOf(uint16(0)):
				var i uint16
				i, err = f.GetUint16(flagName)
				fieldValue.SetUint(uint64(i))
			case reflect.TypeOf((*uint16)(nil)):
				var i uint16
				i, err = f.GetUint16(flagName)
				fieldValue.Set(reflect.ValueOf(&i))
			case reflect.TypeOf(uint32(0)):
				var i uint32
				i, err = f.GetUint32(flagName)
				fieldValue.SetUint(uint64(i))
			case reflect.TypeOf((*uint32)(nil)):
				var i uint32
				i, err = f.GetUint32(flagName)
				fieldValue.Set(reflect.ValueOf(&i))
			case reflect.TypeOf(uint64(0)):
				var i uint64
				i, err = f.GetUint64(flagName)
				fieldValue.SetUint(i)
			case reflect.TypeOf((*uint64)(nil)):
				var i uint64
				i, err = f.GetUint64(flagName)
				fieldValue.Set(reflect.ValueOf(&i))
			case reflect.TypeOf(""):
				var s string
				s, err = f.GetString(flagName)
				fieldValue.SetString(s)
			case reflect.TypeOf(shelly.MQTT_SSL_CA("")):
				var s string
				s, err = f.GetString(flagName)
				fieldValue.Set(reflect.ValueOf(shelly.MQTT_SSL_CA(s)))
			case reflect.TypeOf(shelly.NewNullString("")):
				var s string
				if f.Changed(flagName) {
					s, err = f.GetString(flagName)
					fieldValue.Set(reflect.ValueOf(shelly.NewNullString(s)))
				}
			case reflect.TypeOf((*string)(nil)):
				var s string
				s, err = f.GetString(flagName)
				fieldValue.Set(reflect.ValueOf(&s))
			case reflect.TypeOf([]string{}):
				var s []string
				s, err = f.GetStringArray(flagName)
				fieldValue.Set(reflect.ValueOf(s))
			case reflect.TypeOf([]float64{}):
				var s []float64
				s, err = f.GetFloat64Slice(flagName)
				fieldValue.Set(reflect.ValueOf(s))
			case reflect.TypeOf([]*float64{}):
				var s []float64
				s, err = f.GetFloat64Slice(flagName)
				var sN []*float64
				for _, v := range s {
					v := v
					if math.IsNaN(v) {
						sN = append(sN, (*float64)(nil))
					} else {
						sN = append(sN, &v)
					}
				}
				fieldValue.Set(reflect.ValueOf(sN))
			default:
				return false, fmt.Errorf("unknown type: %v", fieldValue.Type())
			}
			if err != nil {
				return false, err
			}
		}
		return fv.Changed, nil
	}
	return ff
}

func newFlagFactory(f *pflag.FlagSet, method string) fieldFunc {
	var ff fieldFunc
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
		case reflect.Interface:
			return false, nil
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
		case reflect.Slice:
			switch typ {
			case reflect.TypeOf([]string{}):
				desc := fmt.Sprintf(
					"Set the %q field on the %q request.\n--%s mbe specified multiple times for additional values.",
					name,
					method,
					flagName,
				)
				f.StringArray(flagName, nil, desc)
			case reflect.TypeOf([]float64{}):
				desc := fmt.Sprintf(
					"Set the %q field on the %q request.\n--%s may be specified multiple times for additional values.",
					name,
					method,
					flagName,
				)
				f.Float64Slice(flagName, nil, desc)
			case reflect.TypeOf([]*float64{}):
				desc := fmt.Sprintf(
					"Set the %q field on the %q request.\n--%s may be specified multiple times for additional values.\nSet NaN to specify null values.",
					name,
					method,
					flagName,
				)
				f.Float64Slice(flagName, nil, desc)
			case reflect.TypeOf(json.RawMessage(nil)):
				return false, nil
			default:
				return false, fmt.Errorf("unknown slice type: %v", typ)
			}

		default:
			return false, fmt.Errorf("unknown type: %v", typ.Kind())
		}
		return false, nil
	}
	return ff
}
