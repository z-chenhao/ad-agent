package agenthost

import "context"

// Read deployment-independent budget limits at every mutation boundary. Live
// write enablement remains the authority injected at process composition.
func (s *Changes) refreshPolicy(ctx context.Context) error {
	account, err := s.Backend.Account(ctx)
	if err != nil {
		return err
	}
	policy, err := s.Store.BudgetPolicy(ctx, account.Source, s.Policy)
	if err != nil {
		return err
	}
	s.Policy = policy
	return nil
}
