// Copyright 2011 Aaron Jacobs. All Rights Reserved.
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

package oglemock

import (
	"errors"
	"fmt"
	"math"
	"reflect"
)

// PartialExpecation is a function that should be called exactly once with
// expected arguments or matchers in order to set up an expected method call.
// See Controller.ExpectMethodCall below. It returns an expectation that can be
// further modified (e.g. by calling WillOnce).
//
// If the arguments are of the wrong type, the function panics.
type PartialExpecation func(...interface{}) Expectation

// Controller represents an object that implements the central logic of
// oglemock: recording and verifying expectations, responding to mock method
// calls, and so on.
type Controller interface {
	// ExpectCall expresses an expectation that the method of the given name
	// should be called on the supplied mock object. It returns a function that
	// should be called with the expected arguments, matchers for the arguments,
	// or a mix of both.
	//
	// fileName and lineNumber should indicate the line on which the expectation
	// was made, if known.
	//
	// For example:
	//
	//     mockWriter := [...]
	//     controller.ExpectCall(mockWriter, "Write")(ElementsAre(0x1))
	//         .WillOnce(Return(1, nil))
	//
	// If the mock object doesn't have a method of the supplied name, the
	// function panics.
	ExpectCall(
		o MockObject,
		methodName string,
		fileName string,
		lineNumber int) PartialExpecation

	// Finish causes the controller to check for any unsatisfied expectations,
	// and report them as errors if they exist.
	//
	// The controller may panic if any of its methods (including this one) are
	// called after Finish is called.
	Finish()

	// HandleMethodCall looks for a registered expectation matching the call of
	// the given method on mock object o, invokes the appropriate action (if
	// any), and returns the values returned by that action (if any).
	//
	// If the action returns nothing, the controller returns zero values. If
	// there is no matching expectation, the controller reports an error and
	// returns zero values.
	//
	// If the mock object doesn't have a method of the supplied name, the
	// arguments are of the wrong type, or the action returns the wrong types,
	// the function panics.
	//
	// HandleMethodCall is exported for the sake of mock implementations, and
	// should not be used directly.
	HandleMethodCall(
		o MockObject,
		methodName string,
		fileName string,
		lineNumber int,
		args []interface{}) []interface{}
}

// methodMap represents a map from method name to set of expectations for that
// method.
type methodMap map[string][]*InternalExpectation

// objectMap represents a map from mock object ID to a methodMap for that object.
type objectMap map[uintptr]methodMap

// NewController sets up a fresh controller, without any expectations set, and
// configures the controller to use the supplied error reporter.
func NewController(reporter ErrorReporter) Controller {
	return &controllerImpl{reporter, objectMap{}}
}

type controllerImpl struct {
	reporter ErrorReporter
	expectationsByObject objectMap
}

// getExpectations returns the list of registered expectations for the named
// method of the supplied object, or an empty slice if none have been
// registered. When this method returns, it is guaranteed that
// c.expectationsByObject has an entry for the object.
func (c *controllerImpl) getExpectations(
	o MockObject,
	methodName string) []*InternalExpectation {
	id := o.Oglemock_Id()

	// Look up the mock object.
	expectationsByMethod, ok := c.expectationsByObject[id]
	if !ok {
		expectationsByMethod = methodMap{}
		c.expectationsByObject[id] = expectationsByMethod
	}

	result, ok := expectationsByMethod[methodName]
	if !ok {
		return []*InternalExpectation{}
	}

	return result
}

// addExpectation adds an expectation to the list registered for the named
// method of the supplied mock object.
func (c *controllerImpl) addExpectation(
	o MockObject,
	methodName string,
	exp *InternalExpectation) {
	// Get the existing list.
	existing := c.getExpectations(o, methodName)

	// Store a modified list.
	id := o.Oglemock_Id()
	c.expectationsByObject[id][methodName] = append(existing, exp)
}

func (c *controllerImpl) ExpectCall(
	o MockObject,
	methodName string,
	fileName string,
	lineNumber int) PartialExpecation {
	// Find the signature for the requested method.
	oType := reflect.TypeOf(o)
	method, ok := oType.MethodByName(methodName)
	if !ok {
		panic("Unknown method: " + methodName)
	}

	partialAlreadyCalled := false
	return func(args ...interface{}) Expectation {
		// This function should only be called once.
		if partialAlreadyCalled {
			panic("Partial expectation called more than once.")
		}

		partialAlreadyCalled = true

		// Make sure that the number of args is legal. Keep in mind that the
		// method's type has an extra receiver arg.
		if len(args) != method.Type.NumIn() - 1 {
			panic(
				fmt.Sprintf(
					"Expectation for %s given wrong number of arguments: " +
						"expected %d, got %d.",
					methodName,
					method.Type.NumIn() - 1,
					len(args)))
		}

		// Create an expectation and insert it into the controller's map.
		exp := InternalNewExpectation(method.Type, args, fileName, lineNumber)
		c.addExpectation(o, methodName, exp)

		// Return the expectation to the user.
		return exp
	}
}

func (c *controllerImpl) Finish() {
	// Check whether the minimum cardinality for each registered expectation has
	// been satisfied.
	for _, expectationsByMethod := range c.expectationsByObject {
		for methodName, expectations := range expectationsByMethod {
			for _, exp := range expectations {
				minCardinality, _ := computeCardinality(exp)
				if exp.NumMatches < minCardinality {
					c.reporter.ReportError(
						exp.FileName,
						exp.LineNumber,
						errors.New(
							fmt.Sprintf(
								"Unsatisfied expectation; expected %s to be called " +
								"at least %d times; called %d times.",
								methodName,
								minCardinality,
								exp.NumMatches)))
				}
			}
		}
	}
}

// expectationMatches checks the matchers for the expectation against the
// supplied arguments.
func expectationMatches(exp *InternalExpectation, args []interface{}) bool {
	matchers := exp.ArgMatchers
	if len(args) != len(matchers) {
		panic(
			fmt.Sprintf(
				"Wrong number of arguments: expected %d; got %d",
				len(matchers),
				len(args)))
	}

	// Check each matcher.
	for i, matcher := range matchers {
		res, _ := matcher.Matches(args[i])
		if !res {
			return false
		}
	}

	return true
}

// chooseExpectation returns the expectation that matches the supplied
// arguments. If there is more than one such expectation, the one furthest
// along in the list for the method is returned. If there is no such
// expectation, nil is returned.
func (c *controllerImpl) chooseExpectation(
	o MockObject,
	methodName string,
	args []interface{}) *InternalExpectation {
	// Do we have any expectations for this method?
	expectations := c.getExpectations(o, methodName)
	if len(expectations) == 0 {
		return nil
	}

	for i := len(expectations) - 1; i >= 0; i-- {
		if (expectationMatches(expectations[i], args)) {
			return expectations[i]
		}
	}

	return nil
}

// makeZeroReturnValues creates a []interface{} containing appropriate zero
// values for returning from the supplied method type.
func makeZeroReturnValues(method reflect.Method) []interface{} {
	methodType := method.Type
	result := make([]interface{}, methodType.NumOut())

	for i, _ := range result {
		outType := methodType.Out(i)
		zeroVal := reflect.Zero(outType)
		result[i] = zeroVal.Interface()
	}

	return result
}

// computeCardinality decides on the [min, max] range of the number of expected
// matches for the supplied expectations, according to the rules documented in
// expectation.go.
func computeCardinality(exp *InternalExpectation) (min, max uint) {
	// Explicit cardinality.
	if exp.ExpectedNumMatches >= 0 {
		min = uint(exp.ExpectedNumMatches)
		max = min
		return
	}

	// Implicit count based on one-time actions.
	if len(exp.OneTimeActions) != 0 {
		min = uint(len(exp.OneTimeActions))
		max = min

		// If there is a fallback action, this is only a lower bound.
		if exp.FallbackAction != nil {
			max = math.MaxUint32
		}

		return
	}

	// Implicit lack of restriction based on a fallback action being configured.
	if exp.FallbackAction != nil {
		min = 0
		max = math.MaxUint32
		return
	}

	// Implicit cardinality of one.
	min = 1
	max = 1
	return
}

// chooseAction returns the action that should be invoked for the i'th match to
// the supplied expectation (counting from zero). If the implicit "return zero
// values" action should be used, it returns nil.
func chooseAction(i uint, exp *InternalExpectation) Action {
	// Exhaust one-time actions first.
	if i < uint(len(exp.OneTimeActions)) {
		return exp.OneTimeActions[i]
	}

	// Fallback action (or nil if none is configured).
	return exp.FallbackAction
}

func (c *controllerImpl) HandleMethodCall(
	o MockObject,
	methodName string,
	fileName string,
	lineNumber int,
	args []interface{}) []interface{} {
	// Find the signature for the requested method.
	oType := reflect.TypeOf(o)
	method, ok := oType.MethodByName(methodName)
	if !ok {
		panic("Unknown method: " + methodName)
	}

	// Find an expectation matching this call.
	expectation := c.chooseExpectation(o, methodName, args)
	if expectation == nil {
		c.reporter.ReportError(
			fileName,
			lineNumber,
			errors.New(
				fmt.Sprintf("Unexpected call to %s with args: %v", methodName, args)))

		return makeZeroReturnValues(method)
	}

	// Increase the number of matches recorded, and check whether we're over the
	// number expected.
	expectation.NumMatches++
	_, maxCardinality := computeCardinality(expectation)
	if expectation.NumMatches > maxCardinality {
		c.reporter.ReportError(
			expectation.FileName,
			expectation.LineNumber,
			errors.New(
				fmt.Sprintf(
					"Unexpected call to %s: " +
						"expected to be called at most %d times; called %d times.",
					methodName,
					maxCardinality,
					expectation.NumMatches)))

		return makeZeroReturnValues(method)
	}

	// Choose an action to invoke. If there is none, just return zero values.
	action := chooseAction(expectation.NumMatches - 1, expectation)
	if action == nil {
		return makeZeroReturnValues(method)
	}

	// Let the action take over.
	return action.Invoke(args)
}
