// Copyright (C) MongoDB, Inc. 2014-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package mongoimport

import (
	"testing"
	"time"

	"github.com/mongodb/mongo-tools/common/log"
	"github.com/mongodb/mongo-tools/common/options"
	"github.com/mongodb/mongo-tools/common/testtype"
	. "github.com/smartystreets/goconvey/convey"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func init() {
	log.SetVerbosity(&options.Verbosity{
		VLevel: 4,
	})
}

func TestTypedHeaderParser(t *testing.T) {
	testtype.SkipUnlessTestType(t, testtype.UnitTestType)

	Convey(
		"Using 'zip.string(),number.double(),foo.auto(),bar.date(January 2, (2006))'",
		t,
		func() {
			var headers = []string{"zip.string()", "number.double()", "foo.auto()", `bar.date(January 2\, \(2006\))`}
			var colSpecs []ColumnSpec
			var err error

			Convey("with parse grace: auto", func() {
				colSpecs, err = ParseTypedHeaders(headers, pgAutoCast)
				So(colSpecs, ShouldResemble, []ColumnSpec{
					{"zip", new(FieldStringParser), pgAutoCast, "string", []string{"zip"}},
					{"number", new(FieldDoubleParser), pgAutoCast, "double", []string{"number"}},
					{"foo", new(FieldAutoParser), pgAutoCast, "auto", []string{"foo"}},
					{
						"bar",
						&FieldDateParser{"January 2, (2006)"},
						pgAutoCast,
						"date",
						[]string{"bar"},
					},
				})
				So(err, ShouldBeNil)
			})
			Convey("with parse grace: skipRow", func() {
				colSpecs, err = ParseTypedHeaders(headers, pgSkipRow)
				So(colSpecs, ShouldResemble, []ColumnSpec{
					{"zip", new(FieldStringParser), pgSkipRow, "string", []string{"zip"}},
					{"number", new(FieldDoubleParser), pgSkipRow, "double", []string{"number"}},
					{"foo", new(FieldAutoParser), pgSkipRow, "auto", []string{"foo"}},
					{
						"bar",
						&FieldDateParser{"January 2, (2006)"},
						pgSkipRow,
						"date",
						[]string{"bar"},
					},
				})
				So(err, ShouldBeNil)
			})
		},
	)

	Convey("Using various bad headers", t, func() {
		var err error

		Convey("with non-empty arguments for types that don't want them", func() {
			_, err = ParseTypedHeader("zip.string(blah)", pgAutoCast)
			So(err, ShouldNotBeNil)
			_, err = ParseTypedHeader("zip.string(0)", pgAutoCast)
			So(err, ShouldNotBeNil)
			_, err = ParseTypedHeader("zip.int32(0)", pgAutoCast)
			So(err, ShouldNotBeNil)
			_, err = ParseTypedHeader("zip.int64(0)", pgAutoCast)
			So(err, ShouldNotBeNil)
			_, err = ParseTypedHeader("zip.double(0)", pgAutoCast)
			So(err, ShouldNotBeNil)
			_, err = ParseTypedHeader("zip.auto(0)", pgAutoCast)
			So(err, ShouldNotBeNil)
		})
		Convey("with bad arguments for the binary type", func() {
			_, err = ParseTypedHeader("zip.binary(blah)", pgAutoCast)
			So(err, ShouldNotBeNil)
			_, err = ParseTypedHeader("zip.binary(binary)", pgAutoCast)
			So(err, ShouldNotBeNil)
			_, err = ParseTypedHeader("zip.binary(decimal)", pgAutoCast)
			So(err, ShouldNotBeNil)
		})
	})
}

func TestAutoHeaderParser(t *testing.T) {
	testtype.SkipUnlessTestType(t, testtype.UnitTestType)
	Convey("Using 'zip,number'", t, func() {
		var headers = []string{"zip", "number", "foo"}
		var colSpecs = ParseAutoHeaders(headers)
		So(colSpecs, ShouldResemble, []ColumnSpec{
			{"zip", new(FieldAutoParser), pgAutoCast, "auto", []string{"zip"}},
			{"number", new(FieldAutoParser), pgAutoCast, "auto", []string{"number"}},
			{"foo", new(FieldAutoParser), pgAutoCast, "auto", []string{"foo"}},
		})
	})
}

func cast[T any](val any) T {
	converted, ok := val.(T)
	So(ok, ShouldBeTrue)

	return converted
}

func TestFieldParsers(t *testing.T) {
	testtype.SkipUnlessTestType(t, testtype.UnitTestType)

	Convey("Using FieldAutoParser", t, func() {
		var p, _ = NewFieldParser(ctAuto, "")
		var value any
		var err error

		Convey("parses integers when it can", func() {
			value, err = p.Parse("2147483648")
			So(cast[int64](value), ShouldEqual, int64(2147483648))
			So(err, ShouldBeNil)
			value, err = p.Parse("42")
			So(cast[int32](value), ShouldEqual, 42)
			So(err, ShouldBeNil)
			value, err = p.Parse("-2147483649")
			So(cast[int64](value), ShouldEqual, int64(-2147483649))
		})
		Convey("parses decimals when it can", func() {
			value, err = p.Parse("3.14159265")
			So(cast[float64](value), ShouldEqual, 3.14159265)
			So(err, ShouldBeNil)
			value, err = p.Parse("0.123123")
			So(cast[float64](value), ShouldEqual, 0.123123)
			So(err, ShouldBeNil)
			value, err = p.Parse("-123456.789")
			So(cast[float64](value), ShouldEqual, -123456.789)
			So(err, ShouldBeNil)
			value, err = p.Parse("-1.")
			So(cast[float64](value), ShouldEqual, -1.0)
			So(err, ShouldBeNil)
		})
		Convey("leaves everything else as a string", func() {
			value, err = p.Parse("12345-6789")
			So(cast[string](value), ShouldEqual, "12345-6789")
			So(err, ShouldBeNil)
			value, err = p.Parse("06/02/1997")
			So(cast[string](value), ShouldEqual, "06/02/1997")
			So(err, ShouldBeNil)
			value, err = p.Parse("")
			So(cast[string](value), ShouldEqual, "")
			So(err, ShouldBeNil)
		})
	})

	Convey("Using FieldBooleanParser", t, func() {
		var p, _ = NewFieldParser(ctBoolean, "")
		var value interface{}
		var err error

		Convey("parses representations of true correctly", func() {
			value, err = p.Parse("true")
			So(cast[bool](value), ShouldBeTrue)
			So(err, ShouldBeNil)
			value, err = p.Parse("TrUe")
			So(cast[bool](value), ShouldBeTrue)
			So(err, ShouldBeNil)
			value, err = p.Parse("1")
			So(cast[bool](value), ShouldBeTrue)
			So(err, ShouldBeNil)
		})
		Convey("parses representations of false correctly", func() {
			value, err = p.Parse("false")
			So(cast[bool](value), ShouldBeFalse)
			So(err, ShouldBeNil)
			value, err = p.Parse("FaLsE")
			So(cast[bool](value), ShouldBeFalse)
			So(err, ShouldBeNil)
			value, err = p.Parse("0")
			So(cast[bool](value), ShouldBeFalse)
			So(err, ShouldBeNil)
		})
		Convey("does not parse other boolean representations", func() {
			_, err = p.Parse("")
			So(err, ShouldNotBeNil)
			_, err = p.Parse("t")
			So(err, ShouldNotBeNil)
			_, err = p.Parse("f")
			So(err, ShouldNotBeNil)
			_, err = p.Parse("yes")
			So(err, ShouldNotBeNil)
			_, err = p.Parse("no")
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Using FieldBinaryParser", t, func() {
		var value interface{}
		var err error

		Convey("using hex encoding", func() {
			var p, _ = NewFieldParser(ctBinary, "hex")
			Convey("parses valid hex values correctly", func() {
				value, err = p.Parse("400a11")
				So(cast[[]byte](value), ShouldResemble, []byte{64, 10, 17})
				So(err, ShouldBeNil)
				value, err = p.Parse("400A11")
				So(cast[[]byte](value), ShouldResemble, []byte{64, 10, 17})
				So(err, ShouldBeNil)
				value, err = p.Parse("0b400A11")
				So(cast[[]byte](value), ShouldResemble, []byte{11, 64, 10, 17})
				So(err, ShouldBeNil)
				value, err = p.Parse("")
				So(cast[[]byte](value), ShouldResemble, []byte{})
				So(err, ShouldBeNil)
			})
		})
		Convey("using base32 encoding", func() {
			var p, _ = NewFieldParser(ctBinary, "base32")
			Convey("parses valid base32 values correctly", func() {
				value, err = p.Parse("")
				So(cast[[]uint8](value), ShouldResemble, []uint8{})
				So(err, ShouldBeNil)
				value, err = p.Parse("MZXW6YTBOI======")
				So(cast[[]uint8](value), ShouldResemble, []uint8{102, 111, 111, 98, 97, 114})
				So(err, ShouldBeNil)
			})
		})
		Convey("using base64 encoding", func() {
			var p, _ = NewFieldParser(ctBinary, "base64")
			Convey("parses valid base64 values correctly", func() {
				value, err = p.Parse("")
				So(cast[[]uint8](value), ShouldResemble, []uint8{})
				So(err, ShouldBeNil)
				value, err = p.Parse("Zm9vYmFy")
				So(cast[[]uint8](value), ShouldResemble, []uint8{102, 111, 111, 98, 97, 114})
				So(err, ShouldBeNil)
			})
		})
	})

	Convey("Using FieldDateParser", t, func() {
		var value interface{}
		var err error

		Convey("with Go's format", func() {
			var p, _ = NewFieldParser(ctDateGo, "01/02/2006 3:04:05pm MST")
			Convey("parses valid timestamps correctly", func() {
				value, err = p.Parse("01/04/2000 5:38:10pm UTC")
				So(
					cast[time.Time](value),
					ShouldResemble,
					time.Date(2000, 1, 4, 17, 38, 10, 0, time.UTC),
				)
				So(err, ShouldBeNil)
			})
			Convey("does not parse invalid dates", func() {
				_, err = p.Parse("01/04/2000 5:38:10pm")
				So(err, ShouldNotBeNil)
				_, err = p.Parse("01/04/2000 5:38:10 pm UTC")
				So(err, ShouldNotBeNil)
				_, err = p.Parse("01/04/2000")
				So(err, ShouldNotBeNil)
			})
		})
		Convey("with MS's format", func() {
			var p, _ = NewFieldParser(ctDateMS, "MM/dd/yyyy h:mm:sstt")
			Convey("parses valid timestamps correctly", func() {
				value, err = p.Parse("01/04/2000 5:38:10PM")
				So(
					cast[time.Time](value),
					ShouldResemble,
					time.Date(2000, 1, 4, 17, 38, 10, 0, time.UTC),
				)
				So(err, ShouldBeNil)
			})
			Convey("does not parse invalid dates", func() {
				_, err = p.Parse("01/04/2000 :) 05:38:10PM")
				So(err, ShouldNotBeNil)
				_, err = p.Parse("01/04/2000 005:38:10PM")
				So(err, ShouldNotBeNil)
				_, err = p.Parse("01/04/2000 5:38:10 PM")
				So(err, ShouldNotBeNil)
				_, err = p.Parse("01/04/2000")
				So(err, ShouldNotBeNil)
			})
		})
		Convey("with Oracle's format", func() {
			var p, _ = NewFieldParser(ctDateOracle, "mm/Dd/yYYy hh:MI:SsAm")
			Convey("parses valid timestamps correctly", func() {
				value, err = p.Parse("01/04/2000 05:38:10PM")
				So(
					cast[time.Time](value),
					ShouldResemble,
					time.Date(2000, 1, 4, 17, 38, 10, 0, time.UTC),
				)
				So(err, ShouldBeNil)
			})
			Convey("does not parse invalid dates", func() {
				_, err = p.Parse("01/04/2000 :) 05:38:10PM")
				So(err, ShouldNotBeNil)
				_, err = p.Parse("01/04/2000 005:38:10PM")
				So(err, ShouldNotBeNil)
				_, err = p.Parse("01/04/2000 5:38:10 PM")
				So(err, ShouldNotBeNil)
				_, err = p.Parse("01/04/2000")
				So(err, ShouldNotBeNil)
			})
		})
	})

	Convey("Using FieldDoubleParser", t, func() {
		var p, _ = NewFieldParser(ctDouble, "")
		var value interface{}
		var err error

		Convey("parses valid decimal values correctly", func() {
			value, err = p.Parse("3.14159265")
			So(cast[float64](value), ShouldEqual, 3.14159265)
			So(err, ShouldBeNil)
			value, err = p.Parse("0.123123")
			So(cast[float64](value), ShouldEqual, 0.123123)
			So(err, ShouldBeNil)
			value, err = p.Parse("-123456.789")
			So(cast[float64](value), ShouldEqual, -123456.789)
			So(err, ShouldBeNil)
			value, err = p.Parse("-1.")
			So(cast[float64](value), ShouldEqual, -1.0)
			So(err, ShouldBeNil)
		})
		Convey("does not parse invalid numbers", func() {
			_, err = p.Parse("")
			So(err, ShouldNotBeNil)
			_, err = p.Parse("1.1.1")
			So(err, ShouldNotBeNil)
			_, err = p.Parse("1-2.0")
			So(err, ShouldNotBeNil)
			_, err = p.Parse("80-")
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Using FieldInt32Parser", t, func() {
		var p, _ = NewFieldParser(ctInt32, "")
		var value interface{}
		var err error

		Convey("parses valid integer values correctly", func() {
			value, err = p.Parse("2147483647")
			So(cast[int32](value), ShouldEqual, 2147483647)
			So(err, ShouldBeNil)
			value, err = p.Parse("42")
			So(cast[int32](value), ShouldEqual, 42)
			So(err, ShouldBeNil)
			value, err = p.Parse("-2147483648")
			So(cast[int32](value), ShouldEqual, -2147483648)
		})
		Convey("does not parse invalid numbers", func() {
			_, err = p.Parse("")
			So(err, ShouldNotBeNil)
			_, err = p.Parse("42.0")
			So(err, ShouldNotBeNil)
			_, err = p.Parse("1-2")
			So(err, ShouldNotBeNil)
			_, err = p.Parse("80-")
			So(err, ShouldNotBeNil)
			value, err = p.Parse("2147483648")
			So(err, ShouldNotBeNil)
			value, err = p.Parse("-2147483649")
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Using FieldInt64Parser", t, func() {
		var p, _ = NewFieldParser(ctInt64, "")
		var value interface{}
		var err error

		Convey("parses valid integer values correctly", func() {
			value, err = p.Parse("2147483648")
			So(cast[int64](value), ShouldEqual, int64(2147483648))
			So(err, ShouldBeNil)
			value, err = p.Parse("42")
			So(cast[int64](value), ShouldEqual, 42)
			So(err, ShouldBeNil)
			value, err = p.Parse("-2147483649")
			So(cast[int64](value), ShouldEqual, int64(-2147483649))
		})
		Convey("does not parse invalid numbers", func() {
			_, err = p.Parse("")
			So(err, ShouldNotBeNil)
			_, err = p.Parse("42.0")
			So(err, ShouldNotBeNil)
			_, err = p.Parse("1-2")
			So(err, ShouldNotBeNil)
			_, err = p.Parse("80-")
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Using FieldDecimalParser", t, func() {
		var p, _ = NewFieldParser(ctDecimal, "")
		var err error

		Convey("parses valid decimal values correctly", func() {
			for _, ts := range []string{"12235.2355", "42", "0", "-124", "-124.55"} {
				testVal, err := primitive.ParseDecimal128(ts)
				So(err, ShouldBeNil)
				parsedValue, err := p.Parse(ts)
				So(err, ShouldBeNil)

				So(testVal, ShouldResemble, cast[primitive.Decimal128](parsedValue))
			}
		})
		Convey("does not parse invalid decimal values", func() {
			for _, ts := range []string{"", "1-2", "abcd"} {
				_, err = p.Parse(ts)
				So(err, ShouldNotBeNil)
			}
		})
	})

	Convey("Using FieldStringParser", t, func() {
		var p, _ = NewFieldParser(ctString, "")
		var value interface{}
		var err error

		Convey("parses strings as strings only", func() {
			value, err = p.Parse("42")
			So(cast[string](value), ShouldEqual, "42")
			So(err, ShouldBeNil)
			value, err = p.Parse("true")
			So(cast[string](value), ShouldEqual, "true")
			So(err, ShouldBeNil)
			value, err = p.Parse("")
			So(cast[string](value), ShouldEqual, "")
			So(err, ShouldBeNil)
		})
	})

}
