package dis_operations

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockGetDistributedFile struct {
	mock.Mock
}

func (m *MockGetDistributedFile) GetDistributedFile() ([]string, error) {
	args := m.Called()
	return args.Get(0).([]string), args.Error(1)
}

// Mock for deleteFileDefinition.Execute
type MockCommand struct {
	mock.Mock
}

func (m *MockCommand) Execute() error {
	args := m.Called()
	return args.Error(0)
}

func TestDisrm_Success(t *testing.T) {
	// Setup mocks
	mockCmd := new(MockCommand)
	deleteFileDefinition = &cobra.Command{
		Run: func(command *cobra.Command, args []string) {
			// Call the mock's Execute method
			mockCmd.Execute()
		},
	}
	mockCmd.On("Execute").Return(nil)

	// Test case
	args := []string{"picture.jpg"}
	err := Dis_rm(args)

	// Assertions
	assert.NoError(t, err, "Expected no error on successful file deletion")
	mockCmd.AssertCalled(t, "Execute")
}

func TestDisRemove_FileNotFound(t *testing.T) {

	// Test case
	args := []string{"file4"}
	err := Dis_rm(args)

	// Assertions
	assert.Error(t, err, "Expected an error when file is not found")
	assert.Equal(t, "file4 not found", err.Error())
}

func TestDisRemove_ExecutionError(t *testing.T) {
	// Setup mocks
	mockCmd := new(MockCommand)
	deleteFileDefinition = &cobra.Command{}
	mockCmd.On("Execute").Return(errors.New("execution failed"))

	// Test case
	args := []string{"file1"}
	err := Dis_rm(args)

	// Assertions
	assert.Error(t, err, "Expected an error when execution fails")
	assert.Contains(t, err.Error(), "error executing copyCommand")
	mockCmd.AssertCalled(t, "Execute")
}
