/**
 * Sample TypeScript module for testing chunkers.
 */

interface User {
  id: string;
  name: string;
  email: string;
  createdAt: Date;
}

interface UserCreateParams {
  name: string;
  email: string;
}

/**
 * UserManager handles user operations.
 */
class UserManager {
  private users: Map<string, User>;

  constructor() {
    this.users = new Map();
  }

  /**
   * Create a new user.
   */
  createUser(params: UserCreateParams): User {
    const user: User = {
      id: crypto.randomUUID(),
      name: params.name,
      email: params.email,
      createdAt: new Date(),
    };
    this.users.set(user.id, user);
    return user;
  }

  /**
   * Get a user by ID.
   */
  getUser(id: string): User | undefined {
    return this.users.get(id);
  }

  /**
   * Delete a user by ID.
   */
  deleteUser(id: string): boolean {
    return this.users.delete(id);
  }

  /**
   * List all users.
   */
  listUsers(): User[] {
    return Array.from(this.users.values());
  }
}

export { User, UserCreateParams, UserManager };
