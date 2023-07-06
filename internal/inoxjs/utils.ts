export class Defer {
  constructor(readonly functions: (() => void)[] = []) {
  }

  add(fn: () => void) {
    this.functions.push(fn);
  }

  execute() {
    for (let i = this.functions.length - 1; i >= 0; i--) {
        this.functions[i]()
    }
  }
}



export function isError(v: unknown): v is Error {
  return (typeof v == 'object') && v != null && v instanceof Error
}