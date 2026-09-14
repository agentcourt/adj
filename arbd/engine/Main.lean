import AARD.Core

open Lean

def parseJsonInput (input : String) : Except String Json := do
  Json.parse input

def parseStepRequest (j : Json) : Except String StepRequest := do
  fromJson? j

def parseInitializeCaseRequest (j : Json) : Except String InitializeCaseRequest := do
  fromJson? j

def getRequestType (j : Json) : String :=
  match j.getObjVal? "request_type" with
  | .ok value =>
      match value.getStr? with
      | .ok s => s
      | .error _ => ""
  | .error _ => ""

def printJsonLine (j : Json) : IO Unit :=
  IO.println (toString j.compress)

def main (_args : List String) : IO UInt32 := do
  let stdin ← IO.getStdin
  let input ← stdin.readToEnd
  match parseJsonInput input with
  | .error err =>
      printJsonLine (toJson ({ ok := false, error := err } : StepErr))
      pure 1
  | .ok j =>
      match getRequestType j with
      | "initialize_case" =>
          match parseInitializeCaseRequest j with
          | .error err =>
              printJsonLine (toJson ({ ok := false, error := err } : StepErr))
              pure 1
          | .ok req =>
              match initializeCase req with
              | .error err =>
                  printJsonLine (toJson ({ ok := false, error := err } : StepErr))
                  pure 1
              | .ok state =>
                  printJsonLine (toJson ({ ok := true, state := state } : StepOk))
                  pure 0
      | "next_opportunity" =>
          match j.getObjValAs? ArbitrationState "state" with
          | .error err =>
              printJsonLine (toJson ({ ok := false, error := err } : StepErr))
              pure 1
          | .ok state =>
              printJsonLine (toJson (nextOpportunity state))
              pure 0
      | _ =>
          match parseStepRequest j with
          | .error err =>
              printJsonLine (toJson ({ ok := false, error := err } : StepErr))
              pure 1
          | .ok req =>
              match step req with
              | .error err =>
                  printJsonLine (toJson ({ ok := false, error := err } : StepErr))
                  pure 1
              | .ok state =>
                  printJsonLine (toJson ({ ok := true, state := state } : StepOk))
                  pure 0
