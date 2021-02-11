package mm

import (
	"fmt"
	"strings"
	"regexp"
	"net"

	log "github.com/activeshadow/libminimega/minilog"

)

var (
	ipv4Re = regexp.MustCompile(`(?:\d{1,3}[.]){3}\d{1,3}(?:\/\d{1,2})?`)
	stateRe = regexp.MustCompile(`error|quit|running|shutdown|paused`)
	boolOps = regexp.MustCompile(`and|or|not`)
	groups = regexp.MustCompile(`(?:[(][^ ])|(?:[^ ][)])`)
)

type Stack struct {
	s []interface{}
}
	
	

func (s *Stack) Push(item interface{}) {
	s.s = append(s.s,item)

}

func (s *Stack) Pop() interface{} {
	if s.IsEmpty() {
		return nil
	}
	
	lastItem := s.s[len(s.s) - 1]
	s.s = s.s[:len(s.s) -1]
	
	return lastItem

}

func (s *Stack) IsEmpty () bool {
	return len(s.s) == 0

}


type ParseTree struct {	
	left *ParseTree
	right *ParseTree
	term string
	searchFields []string
}

func (node *ParseTree) PrintTree() {

	if node == nil {
		return
	}	
		
	fmt.Printf("Node:%s Fields:%v\n",node.term,node.searchFields)
	
	node.left.PrintTree()
	node.right.PrintTree()
	

}

func BuildTree(searchFilter string) *ParseTree {

	if len(searchFilter) == 0 {
		return nil
	}

	// Adjust any parentheses so that they are 
	// space delimited
	if groups.MatchString(searchFilter) {
		searchFilter = strings.ReplaceAll(searchFilter,"(","( ")
		searchFilter = strings.ReplaceAll(searchFilter,")"," )")		
	}
	
	searchString := strings.ToLower(searchFilter)
	stringParts := strings.Split(searchString," ")

	// If no operators were found, assume a default
	// operator of "and"
	if !boolOps.MatchString(searchString) {
		tmp := strings.Join(stringParts," and ")
		stringParts = strings.Split(tmp," ")
	}

	postFix := postfix(stringParts)
	log.Debug("Postfix:%v",postFix)

	if len(postFix) == 0 {
		return nil
	}

	// If the only term that was passed in
	// is a boolean operator, then skip
	// building the tree
	if len(postFix) == 1 {
		if boolOps.MatchString(postFix[0]) {
			return nil
		}
	}

	parseTree := createTree(postFix)
	
	
	return parseTree

}

func (node *ParseTree) Evaluate(vm *VM) bool {

	if node == nil {
		return false	
	}
	
	if node.left == nil && node.right == nil {
		return node.match(vm)
	
	}
	
	rightSide := false
	if node.right != nil {
		rightSide = node.right.Evaluate(vm)
	}
	
	leftSide := false
	if node.left != nil {
		leftSide = node.left.Evaluate(vm)
	}
	
	switch node.term {
		case "and":			
			return rightSide && leftSide
		
		case "or":			
			return rightSide || leftSide
		
		case "not":			
			return !rightSide 
	}
	
	return false

}

func postfix(terms []string) []string {

	var output []string
	opStack := new(Stack)
	
	for _,term := range terms {
	
		if len(term) == 0 {
			continue
		}
		
		if boolOps.MatchString(term) || term == "(" {
			opStack.Push(term)
		
		} else if term == ")" {
			token := ""
			for token != "(" {
				token = opStack.Pop().(string)
				
				if token != "(" {
					output = append(output,token)
				}
			
			}
			
		
		} else {
		
			output = append(output,term)
		
		}
	
	}
	
	for !opStack.IsEmpty() {
		output = append(output,opStack.Pop().(string))	
	}
	
	return output

}

func createTree(postFix []string) *ParseTree {

	stack := new(Stack)

	for _,term := range postFix {
	
		if boolOps.MatchString(term) {			
			opTree := new(ParseTree)
			opTree.term = term	

			t1 := stack.Pop().(*ParseTree)			
			opTree.right = t1
			
			
			if !stack.IsEmpty() && term != "not" {
				t2 := stack.Pop().(*ParseTree)
				opTree.left = t2								
			}			
			
			stack.Push(opTree)
		
		} else {
		
			operand := new(ParseTree)
			operand.term = term				
			operand.searchFields = getSearchFields(term)
			stack.Push(operand)
		
		}
	
	
	}
	
	return stack.Pop().(*ParseTree)

}


func getSearchFields(term string) []string  {

		
	if ipv4Re.MatchString(term) {		
		return []string{"IPv4"}
	
	} else if stateRe.MatchString(term) {	
		return []string{"State"}
			
	
	} else if strings.Contains(term,"capturing") {	
		return []string{"Captures"}
		
	
	} else if strings.Contains(term,"busy") {
		return []string{"Busy"}

	} else if strings.Contains(term,"dnb") {
		return []string{"DoNotBoot"}

	} else {		
		return []string{"Name","Networks","Host","Disk","Tags"}		
			
	}

}

func (node *ParseTree) match(vm *VM) bool {	
		
	for _,field := range node.searchFields {
		switch field {
			case "IPv4": {				
				_,refNet,err := net.ParseCIDR(node.term)
				
				if err != nil {					
					log.Debug("Unable to parse network:%v",node.term)
					continue
				}		
				
				for _,network := range vm.IPv4 {
				
					address := net.ParseIP(network)		
				
					if address == nil {
						log.Debug("Unable to parse address:%v",network)
						continue
					}
				
					match := refNet.Contains(address)					
					if match {
						return match
					}
					
					
				}
				
				
			}
			case "State": {
				if node.term == "shutdown" || node.term == "quit" {					
					return strings.ToLower(vm.State)== "quit"
				} else {					
					return strings.ToLower(vm.State)== node.term				
				}
			
			
			}
			case "Busy": {				
				
				return vm.Busy
			
			}
			case "Captures": {				
				return len(vm.Captures) > 0				
			}
			case "DoNotBoot": {
				return vm.DoNotBoot
			}
			case "Networks": {
				
				for _,tap := range vm.Networks {
				
					match := strings.Contains(strings.ToLower(tap),node.term)
					if match {
						return match
					}


				}

				continue

				
			}			
			case "Name": {
				
				
				match := strings.Contains(strings.ToLower(vm.Name),node.term)				
				if match {
					return match
				}
				
				continue
			
			}
			case "Host": {
				
				
				match := strings.Contains(strings.ToLower(vm.Host),node.term)				
				if match {
					return match
				}
				
				continue
			
			}			
			case "Tags": {				

				for _,tag := range vm.Tags { 
				
					match := strings.Contains(strings.ToLower(tag),node.term)				
					if match {
						return match
					}
				}

				continue
			}					
			case "Disk": {
				
				match := strings.Contains(strings.ToLower(vm.Disk),node.term)				
				if match {
					return match
				}				
				
				
				continue
							
			}
		}
	}
	
	return false
}


