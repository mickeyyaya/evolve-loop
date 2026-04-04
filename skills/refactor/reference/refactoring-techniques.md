> Read this file during Phase 3 planning. Complete 66-technique Fowler catalog with detection signals, organized by category.

# Refactoring Technique Catalog

Complete catalog of 66 refactoring techniques. Use during Phase 3 to select the right technique for each detected smell.

## Composing Methods (9 Techniques)

| # | Technique | When to Apply | Detection Signal |
|---|-----------|---------------|-----------------|
| 1 | Extract Method | Long method, code comment explaining a block | Function >20 lines or complexity >15 |
| 2 | Inline Method | Method body is as clear as its name | Single-line method called once |
| 3 | Extract Variable | Complex expression hard to understand | Expression with 3+ operators |
| 4 | Inline Temp | Temp variable used once, assigned simple expression | Single-use temp with trivial RHS |
| 5 | Replace Temp with Query | Temp holds a computed value reusable elsewhere | Temp assigned then used in multiple places |
| 6 | Split Temporary Variable | One temp assigned multiple times for different purposes | Variable reassigned with different semantics |
| 7 | Remove Assignments to Parameters | Function parameter is reassigned | Parameter on LHS of assignment |
| 8 | Replace Method with Method Object | Long method with many local variables preventing extraction | >5 local variables in a long method |
| 9 | Substitute Algorithm | Algorithm can be replaced with a clearer one | Complex loop replaceable by built-in or library call |

## Moving Features Between Objects (8 Techniques)

| # | Technique | When to Apply | Detection Signal |
|---|-----------|---------------|-----------------|
| 10 | Move Method | Method uses more features of another class | >50% external references (Feature Envy) |
| 11 | Move Field | Field used more by another class | >50% access from external class |
| 12 | Extract Class | One class doing the work of two | Class has 2+ distinct responsibility clusters |
| 13 | Inline Class | Class does too little | <3 methods, <50 lines (Lazy Class) |
| 14 | Hide Delegate | Client calls through an object to get to another | Message Chain >3 links |
| 15 | Remove Middle Man | Class has too many delegating methods | >50% delegation (Middle Man) |
| 16 | Introduce Foreign Method | Utility method needed on a class you cannot modify | Repeated helper code for external class |
| 17 | Introduce Local Extension | Multiple foreign methods needed for same class | 3+ foreign methods for one class |

## Organizing Data (13 Techniques)

| # | Technique | When to Apply | Detection Signal |
|---|-----------|---------------|-----------------|
| 18 | Self Encapsulate Field | Direct field access causes coupling issues | Subclass needs to override field access |
| 19 | Replace Data Value with Object | Primitive represents a concept with behavior | Primitive + related validation/formatting logic |
| 20 | Change Value to Reference | Many identical objects should be one shared instance | Equality checks on data that should be identity |
| 21 | Change Reference to Value | Reference object is simple and immutable | Small object with no side effects |
| 22 | Replace Array with Object | Array elements mean different things by position | Array with positional semantics (arr[0] = name) |
| 23 | Duplicate Observed Data | Domain data trapped in UI class | UI class holds business state |
| 24 | Change Unidirectional to Bidirectional | Two classes need to reference each other | Class A uses B, and B needs A |
| 25 | Change Bidirectional to Unidirectional | Bidirectional reference no longer needed | One direction is never traversed |
| 26 | Replace Magic Number with Constant | Hardcoded number with special meaning | Numeric literal in condition or calculation |
| 27 | Encapsulate Field | Public field with no access control | Public field on a class |
| 28 | Encapsulate Collection | Getter returns raw collection | Mutable collection returned by reference |
| 29 | Replace Type Code with Class | Type code (int/string) represents a category | String/int constants used in conditionals |
| 30 | Replace Type Code with Subclasses | Type code affects behavior | Switch/if on type code in multiple methods |

## Simplifying Conditional Expressions (8 Techniques)

| # | Technique | When to Apply | Detection Signal |
|---|-----------|---------------|-----------------|
| 31 | Decompose Conditional | Complex conditional expression | If-condition with 3+ clauses |
| 32 | Consolidate Conditional Expression | Multiple conditionals with same result | Adjacent if-blocks returning same value |
| 33 | Consolidate Duplicate Conditional Fragments | Same code in all branches | Identical statements in if and else |
| 34 | Remove Control Flag | Boolean flag controlling loop exit | Variable set to break out of loop |
| 35 | Replace Nested Conditional with Guard Clauses | Deep nesting from sequential checks | Nesting depth >3 from if-chains |
| 36 | Replace Conditional with Polymorphism | Switch on type determines behavior | Switch statement in >1 method on same type |
| 37 | Introduce Null Object | Repeated null checks for same object | >3 null checks for same variable |
| 38 | Introduce Assertion | Code assumes a condition but does not verify it | Implicit precondition without validation |

## Simplifying Method Calls (14 Techniques)

| # | Technique | When to Apply | Detection Signal |
|---|-----------|---------------|-----------------|
| 39 | Rename Method | Name does not reveal intent | Name is generic (process, handle, doStuff) |
| 40 | Add Parameter | Method needs additional data from caller | Caller computes value method should receive |
| 41 | Remove Parameter | Parameter is no longer used | Parameter unused in method body |
| 42 | Separate Query from Modifier | Method both returns value and changes state | Method with return value AND side effects |
| 43 | Parameterize Method | Multiple methods do similar things with different values | 2+ methods differing only in a constant |
| 44 | Replace Parameter with Explicit Methods | Method behavior depends entirely on parameter value | Boolean/enum param selecting code path |
| 45 | Preserve Whole Object | Extracting values from object to pass individually | 3+ fields extracted then passed |
| 46 | Replace Parameter with Method Call | Param value can be obtained by callee | Caller passes value callee can compute itself |
| 47 | Introduce Parameter Object | Group of parameters always passed together | Same 3+ params in multiple signatures |
| 48 | Remove Setting Method | Field should be set only at creation time | Setter called only in constructor |
| 49 | Hide Method | Method only used inside its own class | Zero external callers |
| 50 | Replace Constructor with Factory Method | Complex construction logic or multiple creation paths | Constructor with conditional logic |
| 51 | Replace Error Code with Exception | Method returns error codes | Return value checked for error sentinel |
| 52 | Replace Exception with Test | Exception used for control flow | Try/catch wrapping expected conditions |

## Dealing with Generalization (14 Techniques)

| # | Technique | When to Apply | Detection Signal |
|---|-----------|---------------|-----------------|
| 53 | Pull Up Field | Duplicate field in sibling subclasses | Same field in 2+ subclasses |
| 54 | Pull Up Method | Duplicate method in sibling subclasses | Same method body in 2+ subclasses |
| 55 | Pull Up Constructor Body | Duplicate constructor logic in subclasses | Identical constructor lines in subclasses |
| 56 | Push Down Method | Method only relevant to one subclass | Method used by only 1 of N subclasses |
| 57 | Push Down Field | Field only relevant to one subclass | Field accessed by only 1 of N subclasses |
| 58 | Extract Subclass | Class has features used only in some instances | Fields/methods only used for subset of objects |
| 59 | Extract Superclass | Two classes with similar features | 2 classes sharing 3+ similar methods/fields |
| 60 | Extract Interface | Multiple classes share a subset of methods | Classes used interchangeably for some operations |
| 61 | Collapse Hierarchy | Subclass adds no real behavior | Subclass with 0 additional methods/fields |
| 62 | Form Template Method | Subclasses have methods with same structure but different steps | Similar method outlines in sibling classes |
| 63 | Replace Inheritance with Delegation | Subclass uses only part of parent interface | Refused Bequest — overrides >50% of parent |
| 64 | Replace Delegation with Inheritance | Class delegates everything to another class | Middle Man — >50% pure delegation |
| 65 | Tease Apart Inheritance | One hierarchy serving two responsibilities | Inheritance tree splits along 2 dimensions |
| 66 | Convert Procedural Design to Objects | Procedural code in an OO context | Long functions with data + behavior separated |
