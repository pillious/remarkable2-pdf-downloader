package main

type Stack[T any] []T

func (stack *Stack[T]) isEmpty() bool {
	return len(*stack) == 0
}

func (stack *Stack[T]) Push(element T) {
	*stack = append(*stack, element)
}

func (stack *Stack[T]) Pop() T {
	element := (*stack)[0]
	if len(*stack) > 1 {
		stack = &Stack[T]{}
	} else {
		*stack = (*stack)[1:]
	}
	return element
}
