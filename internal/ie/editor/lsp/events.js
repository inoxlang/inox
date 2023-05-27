"use strict";

export class EventEmitter {
  /** @type {Record<string, Function[]>} */
  _events;

  /**
   *  @param {string} event
   *  @param {Function} listener
   */
  addListener(event, listener) {
    this._events = this._events || {};
    this._events[event] = this._events[event] || [];
    this._events[event].push(listener);
    return this;
  }

  /**
   *  @param {string} event
   *  @param {Function} listener
   */
  on(event, listener) {
    return this.addListener(event, listener);
  }

  /**
   *  @param {string} event
   *  @param {Function} listener
   */
  once(event, listener) {
    function g() {
      this.removeListener(event, g);
      listener.apply(this, arguments);
    }
    this.on(event, g);
    return this;
  }

  /**
   *  @param {string} event
   */
  removeAllListeners(event) {
    if (event) {
      this._events[event] = [];
    } else {
      this._events = {};
    }
    return this;
  }

  /**
   *  @param {string} event
   *  @param {Function} listener
   */
  removeListener(event, listener) {
    this._events = this._events || {};
    if (event in this._events) {
      this._events[event].splice(this._events[event].indexOf(listener), 1);
    }
    return this;
  }

  /**
   *  @param {string} event
   *  @param {Function} listener
   */
  off(event, listener) {
    return this.off(event, listener);
  }

  listeners(event) {
    this._events = this._events || {};
    return this._events[event] || [];
  }

  /**
   *  Execute each of the listeners in order with the supplied arguments.
   *  Returns true if event had listeners, false otherwise.
   *
   *  @param {string} event
   *  @param args The arguments to pass to event listeners.
   */
  emit(event, ...args) {
    this._events = this._events || {};
    // copy array so that removing listeners in listeners (once etc) does not affect the iteration
    var list = (this._events[event] || []).slice(0);
    for (var i = 0; i < list.length; i++) {
      list[i].apply(this, Array.prototype.slice.call(args, 1));
    }
    return list.length > 0;
  }
}
