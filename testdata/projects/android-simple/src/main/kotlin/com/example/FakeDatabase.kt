package com.example

class FakeDatabase {
    private val data = mutableListOf<String>()
    fun insert(item: String) { data.add(item) }
    fun query(): List<String> = data
}
