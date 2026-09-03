package object

type Environment struct {
	store      map[string]Object
	outer      *Environment
	currentDot Object // Holds the value of '.' (current item in pipeline / filter / map)
	File       string // absolute path of the module currently evaluating
	Dir        string // directory of that module (for use-path resolution)
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
		env.File = outer.File
		env.Dir = outer.Dir
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

func (e *Environment) Bindings() map[string]Object {
	return e.store
}
