package daemon

// fatalChan resolves the FatalChan accessor for a component definition.
func fatalChan(def ComponentDefinition, component any) <-chan error {
	if def.FatalChan != nil {
		return def.FatalChan(component)
	}
	return nil
}
