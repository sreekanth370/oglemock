// Copyright 2012 Aaron Jacobs. All Rights Reserved.
// Author: aaronjjacobs@gmail.com (Aaron Jacobs)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package oglemock_test

import (
	. "github.com/jacobsa/oglematchers"
	. "github.com/jacobsa/ogletest"
	"errors"
	"github.com/jacobsa/oglemock"
	"github.com/jacobsa/oglemock/sample/mock_io"
	"runtime"
)

////////////////////////////////////////////////////////////
// Helpers
////////////////////////////////////////////////////////////

func getLineNumber() int {
	_, _, line, _ := runtime.Caller(1)
	return line
}

type IntegrationTest struct {
	reporter fakeErrorReporter
	controller oglemock.Controller

	reader mock_io.MockReader
}

func init() { RegisterTestSuite(&IntegrationTest{}) }

func (t *IntegrationTest) SetUp(c *TestInfo) {
	t.reporter.errorsReported = make([]errorReport, 0)
	t.reporter.fatalErrorsReported = make([]errorReport, 0)
	t.controller = oglemock.NewController(&t.reporter)

	t.reader = mock_io.NewMockReader(t.controller, "")
}

////////////////////////////////////////////////////////////
// Tests
////////////////////////////////////////////////////////////

func (t *IntegrationTest) UnexpectedCall() {
	t.reader.Read([]uint8{1, 2, 3})
	expectedLine := getLineNumber() - 1

	// An error should have been reported.
	AssertEq(1, t.reporter.errorsReported, "%v", t.reporter.errorsReported)
	AssertEq(0, t.reporter.fatalErrorsReported, "%v", t.reporter.fatalErrorsReported)

	r := t.reporter.errorsReported[0]
	ExpectEq("integration_test.go", r.fileName)
	ExpectEq(expectedLine, r.lineNumber)
	ExpectThat(r.err, Error(HasSubstr("Unexpected")))
	ExpectThat(r.err, Error(HasSubstr("Read")))
	ExpectThat(r.err, Error(HasSubstr("1, 2, 3")))
}

func (t *IntegrationTest) ZeroValues() {
	// Make an unexpected call.
	n, err := t.reader.Read([]uint8{})

	// Check the return values.
	ExpectEq(0, n)
	ExpectEq(nil, err)
}

func (t *IntegrationTest) ExpectedCalls() {
	// Expectations
	t.controller.ExpectCall(t.reader, "Read", "", 112)(nil).
		WillOnce(oglemock.Return(17, nil)).
		WillOnce(oglemock.Return(19, nil))

	t.controller.ExpectCall(t.reader, "Read", "", 112)(Not(Equals(nil))).
		WillOnce(oglemock.Return(23, errors.New("taco")))

  // Calls
	var n int
	var err error

	n, err = t.reader.Read(nil)
	ExpectEq(17, n)
	ExpectEq(nil, err)

	n, err = t.reader.Read([]byte{})
	ExpectEq(23, n)
	ExpectThat(err, Error(Equals("taco")))

	n, err = t.reader.Read(nil)
	ExpectEq(19, n)
	ExpectEq(nil, err)

	// Errors
	AssertEq(0, t.reporter.errorsReported, "%v", t.reporter.errorsReported)
	AssertEq(0, t.reporter.fatalErrorsReported, "%v", t.reporter.fatalErrorsReported)
}
