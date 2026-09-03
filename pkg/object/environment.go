package object

type Environment struct {
	store      map[string]Object
	outer      *Environment
	currentDot Object // Holds the value of '.' (current item in pipeline / filter / map)
}

func NewEnvironment() *Environment {
	return &Environment{
		store:      make(map[string]Object),
		outer:      nil,
		currentDot: nil,
	}
}

func NewEnclosedEnvironment(outer *Environment) *Environment {
	env := NewEnvironment()
	env.outer = outer
	if outer != nil {
		env.currentDot = outer.currentDot
	}
	return env
}

func (e *Environment) Get(name string) (Object, bool) {
	obj, ok := e.store[name]
	if !ok && e.outer != nil {
		obj, ok = e.outer.Get(name)
	}
	return obj, ok
}

func (e *Environment) Set(name string, val Object) Object {
	e.store[name] = val
	return val
}

func (e *Environment) SetDot(val Object) {
	e.currentDot = val
}

func (e *Environment) GetDot() Object {
	if e.currentDot != nil {
		return e.currentDot
	}
	if e.outer != nil {
		return e.outer.GetDot()
	}
	return nil
}
