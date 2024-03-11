package css

func MakeClassNameSelector(name string) Node {
	if name[0] != '.' {
		name = "." + name
	}

	return Node{
		Type: Selector,
		Children: []Node{
			{
				Type: ClassName,
				Data: name,
			},
		},
	}
}

func MakeCustomProperty(name, value string) Node {
	if name[0] != '-' {
		name = "--" + name
	}

	return Node{
		Type: CustomProperty,
		Data: name,
		Children: []Node{
			{
				Type: CustomPropertyValue,
				Data: value,
			},
		},
	}
}

func MakeDeclaration(propertyName string, values ...Node) Node {
	return Node{
		Type:     Declaration,
		Data:     propertyName,
		Children: values,
	}
}

func MakeVarCall(name string) Node {
	if name[0] != '-' {
		name = "--" + name
	}

	return Node{
		Type: FunctionCall,
		Data: "var",
		Children: []Node{
			{
				Type: Ident,
				Data: name,
			},
		},
	}
}
