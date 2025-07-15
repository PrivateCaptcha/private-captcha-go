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

func (verr VerifyError) String() string {
	switch verr {
	case VerifyNoError:
		return "no-error"
	case VerifyErrorOther:
		return "error-other"
	case DuplicateSolutionsError:
		return "solution-duplicates"
	case InvalidSolutionError:
		return "solution-invalid"
	case ParseResponseError:
		return "solution-bad-format"
	case PuzzleExpiredError:
		return "puzzle-expired"
	case InvalidPropertyError:
		return "property-invalid"
	case WrongOwnerError:
		return "property-owner-mismatch"
	case VerifiedBeforeError:
		return "solution-verified-before"
	case MaintenanceModeError:
		return "maintenance-mode"
	case TestPropertyError:
		return "property-test"
	case IntegrityError:
		return "integrity-error"
	default:
		return "error"
	}
}

type VerificationResponse struct {
	Success   bool        `json:"success"`
	Code      VerifyError `json:"code"`
	Origin    string      `json:"origin,omitempty"`
	Timestamp string      `json:"timestamp,omitempty"`
}

func (vr *VerificationResponse) Error() string {
	if vr == nil {
		return ""
	}

	return vr.Code.String()
}
