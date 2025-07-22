package testhelpers

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TestTableEntry represents a single test case in a table-driven test
type TestTableEntry struct {
	Description string
	Input       interface{}
	Expected    interface{}
	ShouldError bool
	ErrorMsg    string
	Setup       func()
	Cleanup     func()
}

// RunTableTests runs table-driven tests in Ginkgo style
func RunTableTests(description string, entries []TestTableEntry, testFunc func(entry TestTableEntry)) {
	Describe(description, func() {
		for _, entry := range entries {
			entry := entry // capture range variable
			It(entry.Description, func() {
				if entry.Setup != nil {
					entry.Setup()
				}
				
				if entry.Cleanup != nil {
					defer entry.Cleanup()
				}
				
				testFunc(entry)
			})
		}
	})
}

// ValidationTestCase represents a validation test case
type ValidationTestCase struct {
	Name        string
	Input       interface{}
	ShouldPass  bool
	ErrorFields []string
}

// RunValidationTests runs validation tests
func RunValidationTests(validator func(interface{}) error, testCases []ValidationTestCase) {
	for _, tc := range testCases {
		tc := tc // capture range variable
		It(tc.Name, func() {
			err := validator(tc.Input)
			
			if tc.ShouldPass {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
				for _, field := range tc.ErrorFields {
					Expect(err.Error()).To(ContainSubstring(field))
				}
			}
		})
	}
}

// APITestCase represents an API endpoint test case
type APITestCase struct {
	Name           string
	Method         string
	Path           string
	Body           interface{}
	Headers        map[string]string
	ExpectedStatus int
	ExpectedBody   interface{}
	ExpectedError  string
	Setup          func()
	Validate       func(response interface{})
}

// ErrorTestCase represents an error handling test case
type ErrorTestCase struct {
	Name          string
	Setup         func() error
	ExpectedError string
	ExpectedType  error
}

// RunErrorTests runs error handling tests
func RunErrorTests(testCases []ErrorTestCase) {
	for _, tc := range testCases {
		tc := tc // capture range variable
		It(tc.Name, func() {
			err := tc.Setup()
			
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(tc.ExpectedError))
			
			if tc.ExpectedType != nil {
				Expect(err).To(MatchError(tc.ExpectedType))
			}
		})
	}
}

// BenchmarkCase represents a benchmark test case
type BenchmarkCase struct {
	Name      string
	Size      int
	Setup     func(size int) interface{}
	Operation func(data interface{})
	Cleanup   func()
}

// RunBenchmarks runs benchmark tests
func RunBenchmarks(b *testing.B, cases []BenchmarkCase) {
	for _, bc := range cases {
		b.Run(bc.Name, func(b *testing.B) {
			data := bc.Setup(bc.Size)
			if bc.Cleanup != nil {
				defer bc.Cleanup()
			}
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				bc.Operation(data)
			}
		})
	}
}

// PermutationTestCase generates all permutations of test inputs
type PermutationTestCase struct {
	Name   string
	Inputs [][]interface{}
	Test   func(inputs ...interface{})
}

// RunPermutationTests runs tests with all permutations of inputs
func RunPermutationTests(cases []PermutationTestCase) {
	for _, tc := range cases {
		tc := tc // capture range variable
		Context(tc.Name, func() {
			permutations := generatePermutations(tc.Inputs)
			for i, perm := range permutations {
				perm := perm // capture range variable
				It(fmt.Sprintf("permutation %d", i+1), func() {
					tc.Test(perm...)
				})
			}
		})
	}
}

// generatePermutations generates all permutations of input arrays
func generatePermutations(inputs [][]interface{}) [][]interface{} {
	if len(inputs) == 0 {
		return [][]interface{}{{}}
	}
	
	var result [][]interface{}
	subPerms := generatePermutations(inputs[1:])
	
	for _, value := range inputs[0] {
		for _, subPerm := range subPerms {
			perm := make([]interface{}, 0, len(subPerm)+1)
			perm = append(perm, value)
			perm = append(perm, subPerm...)
			result = append(result, perm)
		}
	}
	
	return result
}

// PropertyTestCase represents a property-based test case
type PropertyTestCase struct {
	Name       string
	Generator  func() interface{}
	Property   func(input interface{}) bool
	NumTests   int
}

// RunPropertyTests runs property-based tests
func RunPropertyTests(cases []PropertyTestCase) {
	for _, tc := range cases {
		tc := tc // capture range variable
		Context(tc.Name, func() {
			numTests := tc.NumTests
			if numTests == 0 {
				numTests = 100
			}
			
			It(fmt.Sprintf("should hold for %d random inputs", numTests), func() {
				for i := 0; i < numTests; i++ {
					input := tc.Generator()
					Expect(tc.Property(input)).To(BeTrue(),
						"Property failed for input: %v", input)
				}
			})
		})
	}
}