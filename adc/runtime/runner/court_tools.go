package runner

func (r *Runner) rule12Grounds() ([]string, error) {
	renderer, err := r.runtimePromptRenderer()
	if err != nil {
		return nil, err
	}
	court, err := renderer.effectiveCourt()
	if err != nil {
		return nil, err
	}
	return rule12Grounds(court), nil
}

func (r *Runner) toolSchema(name string) (map[string]any, error) {
	renderer, err := r.runtimePromptRenderer()
	if err != nil {
		return nil, err
	}
	return renderer.toolSchema(name)
}

func (r *Runner) buildTools(allowed []string) ([]map[string]any, error) {
	renderer, err := r.runtimePromptRenderer()
	if err != nil {
		return nil, err
	}
	return renderer.BuildTools(allowed)
}

func (r *Runner) buildOpportunityTools(allowed []string, reference []string, mayPass bool) ([]map[string]any, error) {
	names := append([]string{}, allowed...)
	for _, name := range reference {
		if !contains(names, name) {
			names = append(names, name)
		}
	}
	if mayPass {
		names = append(names, "pass_turn")
	}
	renderer, err := r.runtimePromptRenderer()
	if err != nil {
		return nil, err
	}
	return renderer.BuildTools(names)
}
