/**
 * Sample JavaScript module for testing chunkers.
 */

/**
 * Counter class that tracks a count value.
 */
class Counter {
  /**
   * Create a new Counter.
   * @param {number} initial - Initial count value.
   */
  constructor(initial = 0) {
    this.count = initial;
  }

  /**
   * Increment the counter.
   * @returns {number} The new count.
   */
  increment() {
    return ++this.count;
  }

  /**
   * Decrement the counter.
   * @returns {number} The new count.
   */
  decrement() {
    return --this.count;
  }

  /**
   * Get the current count.
   * @returns {number} The current count.
   */
  getCount() {
    return this.count;
  }

  /**
   * Reset the counter to zero.
   */
  reset() {
    this.count = 0;
  }
}

/**
 * Create and return a new Counter instance.
 * @param {number} initial - Initial count value.
 * @returns {Counter} A new Counter instance.
 */
function createCounter(initial) {
  return new Counter(initial);
}

module.exports = { Counter, createCounter };
