# ==========================================================
# std/lexer.nl — NeuroLang lexer written in NeuroLang
# Tokens: {type, literal, line, col}
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
  "use": "use",
  "true": "true",
  "false": "false",
  "null": "null"
}

two_ops = ["==", "!=", "<=", ">=", "&&", "||", "->", "??"]

tokenize = code -> {
  tokens = []
  S = {pos: 0, line: 1, col: 1}
  n = len(code)

  while S.pos < n {
    ch = slice(code, S.pos, S.pos + 1)
    if ch == "" {
      break
    }
    if ord(ch) == 65279 {
      S.pos = S.pos + 1
      continue
    }
    tline = S.line
    tcol = S.col

    if is_space(ch) {
      S.pos = S.pos + 1
      if ch == "\n" {
        S.line = S.line + 1
        S.col = 1
      } else {
        S.col = S.col + 1
      }
      continue
    }

    nxt = slice(code, S.pos + 1, S.pos + 2)
    if ch == "#" || (ch == "/" && nxt == "/") {
      while S.pos < n && slice(code, S.pos, S.pos + 1) != "\n" {
        S.pos = S.pos + 1
        S.col = S.col + 1
      }
      continue
    }

    two = ch + nxt
    if two in two_ops {
      tokens = append(tokens, {type: two, literal: two, line: tline, col: tcol})
      S.pos = S.pos + 2
      S.col = S.col + 2
      continue
    }

    if is_digit(ch) {
      start = S.pos
      while S.pos < n && is_digit(slice(code, S.pos, S.pos + 1)) {
        S.pos = S.pos + 1
        S.col = S.col + 1
      }
      if S.pos < n && slice(code, S.pos, S.pos + 1) == "." && is_digit(slice(code, S.pos + 1, S.pos + 2)) {
        S.pos = S.pos + 1
        S.col = S.col + 1
        while S.pos < n && is_digit(slice(code, S.pos, S.pos + 1)) {
          S.pos = S.pos + 1
          S.col = S.col + 1
        }
        tokens = append(tokens, {type: "FLOAT", literal: slice(code, start, S.pos), line: tline, col: tcol})
      } else {
        tokens = append(tokens, {type: "INT", literal: slice(code, start, S.pos), line: tline, col: tcol})
      }
      continue
    }

    if is_alpha(ch) {
      start = S.pos
      while S.pos < n {
        c = slice(code, S.pos, S.pos + 1)
        if is_alpha(c) || is_digit(c) {
          S.pos = S.pos + 1
          S.col = S.col + 1
        } else {
          break
        }
      }
      ident = slice(code, start, S.pos)
      tok_type = keywords[ident]
      if tok_type == null {
        tok_type = "IDENT"
      }
      tokens = append(tokens, {type: tok_type, literal: ident, line: tline, col: tcol})
      continue
    }

    if ch == "\"" || ch == "'" {
      quote = ch
      S.pos = S.pos + 1
      S.col = S.col + 1
      str_val = ""
      while S.pos < n && slice(code, S.pos, S.pos + 1) != quote {
        c = slice(code, S.pos, S.pos + 1)
        if c == "\\" {
          S.pos = S.pos + 1
          S.col = S.col + 1
          e = slice(code, S.pos, S.pos + 1)
          esc = {n: "\n", t: "\t", r: "\r"}
          mapped = esc[e]
          if mapped != null {
            str_val = str_val + mapped
          } else if e == "\\" || e == "\"" || e == "'" {
            str_val = str_val + e
          } else {
            str_val = str_val + e
          }
          S.pos = S.pos + 1
          S.col = S.col + 1
        } else {
          str_val = str_val + c
          S.pos = S.pos + 1
          if c == "\n" {
            S.line = S.line + 1
            S.col = 1
          } else {
            S.col = S.col + 1
          }
        }
      }
      tokens = append(tokens, {type: "STRING", literal: str_val, line: tline, col: tcol})
      if S.pos < n {
        S.pos = S.pos + 1
        S.col = S.col + 1
      }
      continue
    }

    tokens = append(tokens, {type: ch, literal: ch, line: tline, col: tcol})
    S.pos = S.pos + 1
    S.col = S.col + 1
  }

  tokens = append(tokens, {type: "EOF", literal: "", line: S.line, col: S.col})
  tokens
}

{tokenize: tokenize}
