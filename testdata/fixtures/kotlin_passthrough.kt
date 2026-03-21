package com.example

import io.mockk.every
import io.mockk.mockk
import org.junit.Test

class PassThroughTest {
    @MockK lateinit var repo: UserRepository
    
    @Test
    fun testGetUser() {
        val expected = User("Alice")
        every { repo.getUser() } returns expected
        val sut = UserService(repo)
        val result = sut.getUser()
        assertThat(result).isEqualTo(expected)
    }
}
