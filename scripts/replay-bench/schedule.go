package main

// Order returns, for each of n slots, the arms in the order they run in that
// slot. Slot i rotates the arm list left by i, which for two arms is ABBA
// counterbalancing: neither arm is systematically first, and nothing that
// drifts over a run can land on one arm only.
func Order(n int, arms []string) [][]string {
	out := make([][]string, n)
	for i := range n {
		slot := make([]string, len(arms))
		for j := range arms {
			slot[j] = arms[(i+j)%len(arms)]
		}
		out[i] = slot
	}
	return out
}
