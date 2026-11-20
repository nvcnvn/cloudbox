# Developer ergonomics changes

The command line's usage text currently says "Commands are added as their
capability lands; run a command for its own help". That leaves no way to learn
the surface from the binary itself. An agent has to already know the verbs,
and the CLI reference (`user-documentation`) has nothing to be checked
against. This delta makes the command line say what it accepts.

```gherkin
Feature: Developer ergonomics changes

  # @openspec: ADDED
  Rule: The command line lists every command it accepts
    The command line's usage MUST list every command it recognises, and MUST
    NOT list a command it does not recognise, so a program or a reader can
    learn the whole surface from the binary without external documentation.

    Scenario: The usage lists every recognised command
      Given the command line
      When its usage is requested
      Then every command the command line recognises is listed

    Scenario: The usage lists no command the command line rejects
      Given the commands the command line lists in its usage
      When each is requested for its help
      Then none is rejected as an unknown command
```
