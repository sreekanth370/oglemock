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
	"reflect"
)

// Action represents an action to be taken in response to a call to a mock
// method.
type Action interface {
	// Invoke runs the specified action, given the arguments to the mock method.
	// It returns zero or more values that may be treated as the return values of
	// the method. If the action doesn't return any values, it may return the nil
	// slice.
	Invoke(methodArgs []interface{}) []interface{}

	// CheckType returns an error iff the action is not able to deal with methods
	// of the specified type. The type's Kind must be Func.
	CheckType(signature reflect.Type) error
}
