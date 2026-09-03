# ==========================================================
# std/lexer.nl: NeuroLang Lexer written in NeuroLang itself!
# (Demonstrating self-hosting tokenizer capability)
# ==========================================================

keywords = {
  "while": "WHILE",
  "break": "BREAK",
  "continue": "CONTINUE",
  "if": "IF",
  "else": "ELSE",
  "match": "MATCH",
  "return": "RETURN",
  "true": "TRUE",
  "false": "FALSE",
  "null": "NULL"
}

tokenize = code -> {
  tokens = []
  pos = 0
  n = len(code)

  while pos < n {
    ch = slice(code, pos, pos + 1)

    # 1. Skip whitespace
    if is_space(ch) {
      pos = pos + 1
      continue
    }

    # 2. Skip single-line comments (#)
    if ch == "#" {
      while pos < n && slice(code, pos, pos + 1) != "\n" {
        pos = pos + 1
      }
      continue
    }

    # 3. Two-character operators: ==, !=, <=, >=, &&, ||, ->
    next_ch = slice(code, pos + 1, pos + 2)
    two_char = ch + next_ch

    if two_char == "==" || two_char == "!=" || two_char == "<=" || two_char == ">=" || two_char == "&&" || two_char == "||" || two_char == "->" {
      tokens = append(tokens, {type: two_char, literal: two_char})
      pos = pos + 2
      continue
    }

    # 4. Numbers (Integer / Float)
    if is_digit(ch) {
      start = pos
      while pos < n && is_digit(slice(code, pos, pos + 1)) {
        pos = pos + 1
      }
      if pos < n && slice(code, pos, pos + 1) == "." && is_digit(slice(code, pos + 1, pos + 2)) {
        pos = pos + 1
        while pos < n && is_digit(slice(code, pos, pos + 1)) {
          pos = pos + 1
        }
        tokens = append(tokens, {type: "FLOAT", literal: slice(code, start, pos)})
      } else {
        tokens = append(tokens, {type: "INT", literal: slice(code, start, pos)})
      }
      continue
    }

    # 5. Identifiers and Keywords
    if is_alpha(ch) {
      start = pos
      while pos < n {
        c = slice(code, pos, pos + 1)
        if is_alpha(c) || is_digit(c) {
          pos = pos + 1
        } else {
          break
        }
      }
      ident = slice(code, start, pos)
      tok_type = keywords[ident]
      if tok_type == null {
        tok_type = "IDENT"
      }
      tokens = append(tokens, {type: tok_type, literal: ident})
      continue
    }

    # 6. Strings ("..." or '...')
    if ch == "\"" || ch == "'" {
      quote = ch
      pos = pos + 1
      start = pos
      while pos < n && slice(code, pos, pos + 1) != quote {
        pos = pos + 1
      }
      str_val = slice(code, start, pos)
      tokens = append(tokens, {type: "STRING", literal: str_val})
      pos = pos + 1 # skip closing quote
      continue
    }

    # 7. Single-character operators and delimiters
    # AI Combinators: |, ?, @, &, !, .
    tokens = append(tokens, {type: ch, literal: ch})
    pos = pos + 1
  }

  tokens = append(tokens, {type: "EOF", literal: ""})
  tokens
}

# ==========================================================
# Self-Verification Test
# ==========================================================
sample_code = "users | ?(.age >= 18) | @.name | !print"

print("--- Source Code to Tokenize ---")
print(sample_code)
print("")

tokens = tokenize(sample_code)

print("--- Generated Token Stream (Self-Hosted Lexer) ---")
tokens | @("  [" + .type + "] => " + .literal) | @(print(.))
print("")
print("Total Tokens:", len(tokens))
