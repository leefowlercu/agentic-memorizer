//! Sample Rust module for testing chunkers.

use std::collections::HashMap;

/// A simple key-value store.
pub struct Store {
    data: HashMap<String, String>,
}

impl Store {
    /// Create a new empty store.
    pub fn new() -> Self {
        Store {
            data: HashMap::new(),
        }
    }

    /// Set a key-value pair.
    pub fn set(&mut self, key: &str, value: &str) {
        self.data.insert(key.to_string(), value.to_string());
    }

    /// Get a value by key.
    pub fn get(&self, key: &str) -> Option<&String> {
        self.data.get(key)
    }

    /// Delete a key-value pair.
    pub fn delete(&mut self, key: &str) -> bool {
        self.data.remove(key).is_some()
    }

    /// Check if a key exists.
    pub fn contains(&self, key: &str) -> bool {
        self.data.contains_key(key)
    }

    /// Get the number of entries.
    pub fn len(&self) -> usize {
        self.data.len()
    }

    /// Check if the store is empty.
    pub fn is_empty(&self) -> bool {
        self.data.is_empty()
    }
}

impl Default for Store {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_set_and_get() {
        let mut store = Store::new();
        store.set("key", "value");
        assert_eq!(store.get("key"), Some(&"value".to_string()));
    }

    #[test]
    fn test_delete() {
        let mut store = Store::new();
        store.set("key", "value");
        assert!(store.delete("key"));
        assert!(store.get("key").is_none());
    }
}
