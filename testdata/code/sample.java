package com.example.sample;

import java.util.ArrayList;
import java.util.List;

/**
 * Sample Java class for testing chunkers.
 */
public class TaskManager {
    private List<Task> tasks;

    public TaskManager() {
        this.tasks = new ArrayList<>();
    }

    /**
     * Add a new task.
     * @param title The task title.
     * @param description The task description.
     * @return The created task.
     */
    public Task addTask(String title, String description) {
        Task task = new Task(tasks.size() + 1, title, description);
        tasks.add(task);
        return task;
    }

    /**
     * Get a task by ID.
     * @param id The task ID.
     * @return The task, or null if not found.
     */
    public Task getTask(int id) {
        return tasks.stream()
            .filter(t -> t.getId() == id)
            .findFirst()
            .orElse(null);
    }

    /**
     * Mark a task as complete.
     * @param id The task ID.
     * @return true if the task was found and marked complete.
     */
    public boolean completeTask(int id) {
        Task task = getTask(id);
        if (task != null) {
            task.setComplete(true);
            return true;
        }
        return false;
    }

    /**
     * Get all tasks.
     * @return List of all tasks.
     */
    public List<Task> getAllTasks() {
        return new ArrayList<>(tasks);
    }
}

/**
 * Represents a task.
 */
class Task {
    private int id;
    private String title;
    private String description;
    private boolean complete;

    public Task(int id, String title, String description) {
        this.id = id;
        this.title = title;
        this.description = description;
        this.complete = false;
    }

    public int getId() { return id; }
    public String getTitle() { return title; }
    public String getDescription() { return description; }
    public boolean isComplete() { return complete; }
    public void setComplete(boolean complete) { this.complete = complete; }
}
