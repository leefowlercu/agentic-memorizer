/**
 * Sample C file for testing chunkers.
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define MAX_NAME_LEN 64

/**
 * Person structure.
 */
typedef struct {
    char name[MAX_NAME_LEN];
    int age;
} Person;

/**
 * Create a new person.
 * @param name The person's name.
 * @param age The person's age.
 * @return A pointer to the new Person, or NULL on failure.
 */
Person* person_create(const char* name, int age) {
    Person* p = (Person*)malloc(sizeof(Person));
    if (p == NULL) {
        return NULL;
    }
    strncpy(p->name, name, MAX_NAME_LEN - 1);
    p->name[MAX_NAME_LEN - 1] = '\0';
    p->age = age;
    return p;
}

/**
 * Free a person structure.
 * @param p The person to free.
 */
void person_free(Person* p) {
    if (p != NULL) {
        free(p);
    }
}

/**
 * Print person information.
 * @param p The person to print.
 */
void person_print(const Person* p) {
    if (p != NULL) {
        printf("Name: %s, Age: %d\n", p->name, p->age);
    }
}

/**
 * Check if person is adult.
 * @param p The person to check.
 * @return 1 if adult, 0 otherwise.
 */
int person_is_adult(const Person* p) {
    if (p == NULL) {
        return 0;
    }
    return p->age >= 18;
}
