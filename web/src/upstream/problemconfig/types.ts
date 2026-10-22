// @ts-nocheck
export interface TestCaseConfig {
  input: string;
  output: string;
  time?: string;
  memory?: string;
  score?: number;
}

export interface SubtaskConfig {
  id?: number;
  score?: number;
  time?: string;
  memory?: string;
  type?: 'min' | 'max' | 'sum';
  if?: number[];
  cases: TestCaseConfig[];
}

export interface ProblemConfigFile {
  type?: string;
  subType?: string;
  target?: string;
  score?: number;
  checker_type?: string;
  checker?: string | { file: string; lang: string };
  interactor?: string | { file: string; lang: string };
  manager?: string | { file: string; lang: string };
  filename?: string;
  time?: string;
  memory?: string;
  num_processes?: number;
  multi_pass?: number;
  answers?: Record<string, unknown>;
  user_extra_files?: string[];
  judge_extra_files?: string[];
  langs?: string[];
  subtasks?: SubtaskConfig[];
  __valid?: boolean;
  __errors?: string[];
  __cases?: TestCaseConfig[];
  [key: string]: unknown;
}
