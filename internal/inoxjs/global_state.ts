import { Context } from "./context.ts";
import { Value } from "./value.ts";

export class GlobalState {
    readonly globals = new Map<string, Value>()

    constructor(readonly ctx: Context){

    }
}

export function newGlobalState(ctx: Context): GlobalState {
    return new GlobalState(ctx)
}