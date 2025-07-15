package privatecaptcha

type VerifyError int

const (
	VerifyNoError           VerifyError = 0
	VerifyErrorOther        VerifyError = 1
	DuplicateSolutionsError VerifyError = 2
	InvalidSolutionError    VerifyError = 3
	ParseResponseError      VerifyError = 4
	PuzzleExpiredError      VerifyError = 5
	InvalidPropertyError    VerifyError = 6
	WrongOwnerError         VerifyError = 7
	VerifiedBeforeError     VerifyError = 8
	MaintenanceModeError    VerifyError = 9
	TestPropertyError       VerifyError = 10
	IntegrityError          VerifyError = 11
	// Add new fields _above_
	VERIFY_ERRORS_COUNT
)

type VerificationResponse struct {
	Success   bool        `json:"success"`
	Code      VerifyError `json:"code"`
	Origin    string      `json:"origin,omitempty"`
	Timestamp string      `json:"timestamp,omitempty"`
}
