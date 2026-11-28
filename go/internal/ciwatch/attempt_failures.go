package ciwatch

import "context"

func EarlierAttemptFailures(ctx context.Context, repoRoot string, runID int64, attempt int) ([]FailingTest, error) {
	tries, err := ghClassifySource{root: repoRoot}.earlierAttempts(ctx, runID, attempt)
	if err != nil {
		return nil, err
	}
	var all []FailingTest
	for _, st := range tries {
		all = append(all, st.FailingTests...)
	}
	return all, nil
}
