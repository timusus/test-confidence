package com.example

class UserViewModel(private val repo: UserRepository) {
    fun getUser() = repo.getUser()
    fun getUserName() = repo.getUser().name
    fun computeSomething(): Int {
        val user = repo.getUser()
        return if (user != null) user.age else 0
    }
}
