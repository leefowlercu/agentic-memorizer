/**
 * Sample C++ file for testing chunkers.
 */

#include <iostream>
#include <string>
#include <vector>
#include <memory>

/**
 * Base class for shapes.
 */
class Shape {
public:
    virtual ~Shape() = default;
    virtual double area() const = 0;
    virtual std::string name() const = 0;
};

/**
 * Circle implementation.
 */
class Circle : public Shape {
private:
    double radius_;

public:
    explicit Circle(double radius) : radius_(radius) {}

    double area() const override {
        return 3.14159265359 * radius_ * radius_;
    }

    std::string name() const override {
        return "Circle";
    }

    double radius() const { return radius_; }
};

/**
 * Rectangle implementation.
 */
class Rectangle : public Shape {
private:
    double width_;
    double height_;

public:
    Rectangle(double width, double height) : width_(width), height_(height) {}

    double area() const override {
        return width_ * height_;
    }

    std::string name() const override {
        return "Rectangle";
    }

    double width() const { return width_; }
    double height() const { return height_; }
};

/**
 * ShapeCollection manages a collection of shapes.
 */
class ShapeCollection {
private:
    std::vector<std::unique_ptr<Shape>> shapes_;

public:
    void add(std::unique_ptr<Shape> shape) {
        shapes_.push_back(std::move(shape));
    }

    double totalArea() const {
        double total = 0;
        for (const auto& shape : shapes_) {
            total += shape->area();
        }
        return total;
    }

    size_t count() const {
        return shapes_.size();
    }
};
