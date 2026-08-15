package expense_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestExpense(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Expense Suite")
}
