package main

type Stack[T any] []T

func (stack *Stack[T]) isEmpty() bool {
	return len(*stack) == 0
}

func (stack *Stack[T]) Push(element T) {
	*stack = append(*stack, element)
}

// Assumes len(stack) > 0
func (stack *Stack[T]) Pop() T {
	l := len(*stack)
	element := (*stack)[l-1]
	if l > 1 {
		*stack = (*stack)[:l-1]
	} else {
		*stack = Stack[T]{}
	}
	return element
}
