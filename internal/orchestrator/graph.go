package orchestrator

import "fmt"

func Sort(nodes []Node) ([]Node, error) {
	nodeMap := make(map[string]Node)
	for _, node := range nodes {
		nodeMap[node.GetName()] = node
	}

	var sorted []Node
	visited := make(map[string]bool)
	recursionStack := make(map[string]bool)

	var visit func(name string) error
	visit = func(name string) error {
		node, ok := nodeMap[name]
		if !ok {
			return fmt.Errorf("обнаружена зависимость от несуществующего сервиса/базы: '%s'", name)
		}

		if recursionStack[name] {
			return fmt.Errorf("обнаружен цикл зависимостей: '%s'", name)
		}

		if visited[name] {
			return nil
		}

		visited[name] = true
		recursionStack[name] = true

		for _, depName := range node.GetDependencies() {
			if err := visit(depName); err != nil {
				return err
			}
		}
		delete(recursionStack, name)
		sorted = append(sorted, node)
		return nil
	}

	for _, node := range nodes {
		if err := visit(node.GetName()); err != nil {
			return nil, err
		}
	}

	return sorted, nil
}
