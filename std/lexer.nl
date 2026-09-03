# ==========================================================
# std/lexer.nl — NeuroLang lexer written in NeuroLang
# Token types match the Go host lexer (keywords keep lowercase types).
# ==========================================================

keywords = {
  "while": "while",
  "for": "for",
  "in": "in",
  "break": "break",
  "continue": "continue",
  "if": "if",
  "else": "else",
  "match": "match",
  "return": "return",
  "fn": "fn",
  "true": "true",
  "false": "false",
  "null": "null"
}

two_ops = ["==", "!=", "<=", ">=", "&&", "||", "->"]

tokenize = code -> {
  tokens = []
  pos = 0
  n = len(code)

  while pos < n {
    ch = slice(code, pos, pos + 1)

    # whitespace
    if is_space(ch) {
      pos = pos + 1
      continue
    }

    # comments: # ... or // ...
    nxt = slice(code, pos + 1, pos + 2)
    if ch == "#" || (ch == "/" && nxt == "/") {
      while pos < n && slice(code, pos, pos + 1) != "\n" {
        pos = pos + 1
      }
      continue
    }

    # two-character operators
    two = ch + nxt
    if two in two_ops {
      tokens = append(tokens, {type: two, literal: two})
      pos = pos + 2
      continue
    }

    # numbers
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

    # identifiers / keywords
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

    # strings with escapes
    if ch == "\"" || ch == "'" {
      quote = ch
      pos = pos + 1
      str_val = ""
      while pos < n && slice(code, pos, pos + 1) != quote {
        c = slice(code, pos, pos + 1)
        if c == "\\" {
          pos = pos + 1
          e = slice(code, pos, pos + 1)
          esc = {n: "\n", t: "\t", r: "\r"}
          mapped = esc[e]
          if mapped != null {
            str_val = str_val + mapped
          } else if e == "\\" || e == "\"" || e == "'" {
            str_val = str_val + e
          } else {
            str_val = str_val + e
          }
          pos = pos + 1
        } else {
          str_val = str_val + c
          pos = pos + 1
        }
      }
      tokens = append(tokens, {type: "STRING", literal: str_val})
      pos = pos + 1
      continue
    }

    tokens = append(tokens, {type: ch, literal: ch})
    pos = pos + 1
  }

  tokens = append(tokens, {type: "EOF", literal: ""})
  tokens
}
